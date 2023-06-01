/*
  Copyright @ 2021 bocloud <fushaosong@beyondcent.com>.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package volume

import (
	"context"
	"errors"
	"fmt"
	"github.com/carina-io/carina"
	"strings"
	"time"

	"github.com/carina-io/carina/api"

	"github.com/carina-io/carina/pkg/devicemanager/bcache"
	"github.com/carina-io/carina/pkg/devicemanager/lvmd"
	"github.com/carina-io/carina/pkg/devicemanager/types"
	"github.com/carina-io/carina/utils/log"
	"github.com/carina-io/carina/utils/mutx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	VOLUMEMUTEX = "VolumeMutex"
)

type LocalVolumeImplement struct {
	Lv     lvmd.Lvm2
	Bcache bcache.Bcache
	Mutex  *mutx.GlobalLocks
}

func (v *LocalVolumeImplement) CreateVolume(lvName, vgName string, size, ratio uint64) error {
	if !v.Mutex.TryAcquire(VOLUMEMUTEX) {
		log.Info("wait other task release mutex, please retry...")
		return errors.New("get global mutex failed")
	}
	defer v.Mutex.Release(VOLUMEMUTEX)

	vgInfo, err := v.Lv.VGDisplay(vgName)
	if err != nil {
		log.Errorf("get device group info failed %s %s", vgName, err.Error())
		return err
	}
	if vgInfo == nil {
		log.Error("cannot find device group info")
		return errors.New("cannot find device group info")
	}

	if vgInfo.VGFree-size < carina.DefaultReservedSpace-carina.DefaultEdgeSpace { ////avoid edge conditions
		log.Warnf("%s don't have enough space, reserved 10g", vgName)
		return errors.New(carina.ResourceExhausted)
	}

	name := carina.VolumePrefix + lvName

	lvInfo, _ := v.Lv.LVDisplay(name, vgName)
	if lvInfo != nil && lvInfo.VGName == vgName {
		log.Infof("%s/%s volume exists", vgName, name)
		return nil
	}

	// 创建volume卷
	return v.Lv.LVCreateFromVG(name, vgName, size, []string{}, 0, "")
}

func (v *LocalVolumeImplement) DeleteVolume(lvName, vgName string) error {
	if !v.Mutex.TryAcquire(VOLUMEMUTEX) {
		log.Info("wait other task release mutex, please retry...")
		return errors.New("get global mutex failed")
	}
	defer v.Mutex.Release(VOLUMEMUTEX)

	name := lvName
	if !strings.HasPrefix(lvName, carina.VolumePrefix) {
		name = carina.VolumePrefix + lvName
	}

	lvInfo, err := v.Lv.LVDisplay(name, vgName)
	if err != nil && strings.Contains(err.Error(), "not found") {
		log.Warnf("volume %s/%s not exist", vgName, lvName)
		return nil
	}
	if err != nil {
		log.Errorf("get volume failed %s/%s %s", vgName, lvName, err.Error())
		return err
	}
	// delete bcache device if exists
	_ = v.DeleteBcache(fmt.Sprintf("/dev/%s/%s", vgName, name), "")

	if err := v.Lv.LVRemove(name, vgName); err != nil {
		return err
	}

	// backward compatible
	thinInfo, _ := v.Lv.LVDisplay(lvInfo.PoolLV, vgName)
	if thinInfo == nil {
		log.Error("cannot find thin info")
		return nil
	}
	return v.Lv.DeleteThinPool(lvInfo.PoolLV, vgName)
}

func (v *LocalVolumeImplement) ResizeVolume(lvName, vgName string, size, ratio uint64) error {
	if !v.Mutex.TryAcquire(VOLUMEMUTEX) {
		log.Info("wait other task release mutex, please retry...")
		return errors.New("get global mutex failed")
	}
	defer v.Mutex.Release(VOLUMEMUTEX)

	// vg 检查
	vgInfo, err := v.Lv.VGDisplay(vgName)
	if err != nil {
		log.Errorf("get device group info failed %s %s", vgName, err.Error())
		return err
	}
	if vgInfo == nil {
		log.Error("cannot find device group info")
		return errors.New("cannot find device group info")
	}

	name := carina.VolumePrefix + lvName

	lvInfo, err := v.Lv.LVDisplay(name, vgName)
	if err != nil {
		log.Errorf("get volume info failed %s/%s %s", vgName, name, err.Error())
		return nil
	}
	if lvInfo == nil {
		log.Infof("%s/%s volume don't exists", vgName, name)
		return errors.New("volume don't exists")
	}

	if lvInfo.LVSize == size {
		log.Infof("%s/%s have expend", vgName, lvName)
		return nil
	}

	if vgInfo.VGFree-(size-lvInfo.LVSize) < carina.DefaultReservedSpace-carina.DefaultEdgeSpace { //avoid edge conditions
		log.Warnf("%s don't have enough space, reserved 10g", vgName)
		return errors.New(carina.ResourceExhausted)
	}

	// backward compatible
	thinInfo, _ := v.Lv.LVDisplay(lvInfo.PoolLV, vgName)
	if thinInfo != nil && thinInfo.LVSize < size {
		if err := v.Lv.ResizeThinPool(lvInfo.PoolLV, vgName, size*ratio); err != nil {
			return err
		}
	}

	return v.Lv.LVResize(name, vgName, size)
}

func (v *LocalVolumeImplement) VolumeList(lvName, vgName string) ([]types.LvInfo, error) {
	name := ""
	if lvName != "" && vgName != "" {
		name = fmt.Sprintf("%s/%s", vgName, lvName)
	}
	return v.Lv.LVS(name)
}

func (v *LocalVolumeImplement) VolumeInfo(lvName, vgName string) (*types.LvInfo, error) {
	lvs, err := v.VolumeList(lvName, vgName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list lv :%v", err)
	}

	for _, v := range lvs {
		if v.LVName == lvName {
			return &v, nil
		}
	}

	return nil, errors.New("not found")
}

func (v *LocalVolumeImplement) GetCurrentVgStruct() ([]api.VgGroup, error) {

	resp := []api.VgGroup{}
	tmp := map[string]*api.VgGroup{}

	vgs, err := v.Lv.VGS()
	if err != nil {
		return nil, err
	}
	for i, v := range vgs {
		tmp[v.VGName] = &vgs[i]
	}

	// 过滤属于VG的PV
	pvs, err := v.Lv.PVS()
	if err != nil {
		return nil, err
	}
	for i, v := range pvs {
		if v.VGName == "" {
			continue
		}
		if tmp[v.VGName] != nil {
			tmp[v.VGName].PVS = append(tmp[v.VGName].PVS, &pvs[i])
		}
	}

	for k, _ := range tmp {
		resp = append(resp, *tmp[k])
	}
	return resp, nil
}

func (v *LocalVolumeImplement) GetCurrentPvStruct() ([]api.PVInfo, error) {
	return v.Lv.PVS()
}

func (v *LocalVolumeImplement) AddNewDiskToVg(disk, vgName string) error {
	vgName = strings.ToLower(vgName)
	if !v.Mutex.TryAcquire(VOLUMEMUTEX) {
		log.Info("wait other task release mutex, please retry...")
		return errors.New("get global mutex failed")
	}
	defer v.Mutex.Release(VOLUMEMUTEX)
	// 确保PV存在
	pvInfo, err := v.Lv.PVDisplay(disk)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		log.Infof("get pv detail failed %s", err.Error())
		return err
	}
	if pvInfo == nil {
		err = v.Lv.PVCreate(disk)
		if err != nil {
			log.Errorf("create pv failed %s", disk)
			return err
		}
	} else {
		if pvInfo.VGName != "" {
			log.Errorf("pv %s have bind vg %s ", pvInfo.PVName, pvInfo.VGName)
			return fmt.Errorf("pv %s have bind vg %s ", pvInfo.PVName, pvInfo.VGName)
		}
	}
	// 检查PV,决定新创建还是扩容
	vgInfo, err := v.Lv.VGDisplay(vgName)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		log.Errorf("get vg detail failed %s", err.Error())
		return err
	}
	if vgInfo == nil {
		err = v.Lv.VGCreate(vgName, []string{vgName}, []string{disk})
		if err != nil {
			log.Errorf("vg create failed %s", err.Error())
			return err
		}
	} else {
		err = v.Lv.VGExtend(vgName, disk)
		if err != nil {
			log.Errorf("vg extend failed %s", err.Error())
			return err
		}
	}

	return nil
}
func (v *LocalVolumeImplement) RemoveDiskInVg(disk, vgName string) error {
	if !v.Mutex.TryAcquire(VOLUMEMUTEX) {
		log.Info("wait other task release mutex, please retry...")
		return errors.New("get global mutex failed")
	}
	defer v.Mutex.Release(VOLUMEMUTEX)

	// 确保PV存在
	pvInfo, err := v.Lv.PVDisplay(disk)
	if err != nil {
		log.Infof("get pv %s detail failed %s", disk, err.Error())
		return err
	}
	if pvInfo == nil {
		log.Warnf("this pv not found %s", disk)
		return nil
	} else {
		if pvInfo.VGName != vgName {
			log.Errorf("pv %s have bind vg %s not %s", pvInfo.PVName, pvInfo.VGName, vgName)
			return fmt.Errorf("pv %s have bind vg %s not %s ", pvInfo.PVName, pvInfo.VGName, vgName)
		}
		if pvInfo.VGName == "" {
			err = v.Lv.PVRemove(disk)
			if err != nil {
				log.Errorf("remove pv failed %s", disk)
				return err
			}
			return nil
		}
	}
	// 获取Vg信息
	vgInfo, err := v.Lv.VGDisplay(vgName)
	if err != nil {
		log.Errorf("get vg %s detail failed %s", vgName, err.Error())
		return err
	}
	if vgInfo == nil {
		log.Errorf("vg %s not found", vgName)
		return errors.New("not found")
	} else {
		// 当vg卷下只有一个pv时，需要检查是否还存在lv
		if vgInfo.PVCount == 1 {
			if vgInfo.LVCount > 0 {
				log.Warnf("cannot remove the disk %s because there are still have logic volumes", disk)
				return errors.New("still have logical volumes")
			}
			if err = v.Lv.VGRemove(vgName); err != nil {
				log.Errorf("vg remove %s failed %s", vgName, err.Error())
				return err
			}
			if err = v.Lv.PVRemove(disk); err != nil {
				log.Errorf("pv remove %s failed %s", disk, err.Error())
				return err
			}
		} else {
			// 移除该Pv，剩余空间不足，则不允许移除
			if vgInfo.VGFree-carina.DefaultReservedSpace+carina.DefaultEdgeSpace < pvInfo.PVSize {
				log.Warnf("cannot remove the disk %s because there will not enough space", disk)
				return errors.New(carina.ResourceExhausted)
			}

			if err = v.Lv.VGReduce(vgName, disk); err != nil {
				log.Errorf("vgreduce %s from vg %s failed %s", disk, vgName, err.Error())
				return err
			}
		}
	}
	return nil
}

func (v *LocalVolumeImplement) HealthCheck() {
	if !v.Mutex.TryAcquire(VOLUMEMUTEX) {
		log.Info("wait other task release mutex, please retry...")
		return
	}
	defer v.Mutex.Release(VOLUMEMUTEX)

	ctx, cf := context.WithTimeout(context.TODO(), 25*time.Second)
	defer cf()

	for {
		select {
		case <-ctx.Done():
			log.Info("volume health check timeout.")
		default:
			_ = v.Lv.RemoveUnknownDevice(carina.DeviceVGHDD)
			_ = v.Lv.RemoveUnknownDevice(carina.DeviceVGSSD)
			return
		}
	}
}

func (v *LocalVolumeImplement) RefreshLvmCache() {
	// start lvmpolld
	_ = v.Lv.StartLvm2()

	// 刷新缓存
	if err := v.Lv.PVScan(""); err != nil {
		log.Warnf(" error during pvscan: %v", err)
	}

	if err := v.Lv.VGScan(""); err != nil {
		log.Warnf("error during vgscan: %v", err)
	}

}

// CreateBcache bcache
func (v *LocalVolumeImplement) CreateBcache(dev, cacheDev string, block, bucket string, cachePolicy string) (*types.BcacheDeviceInfo, error) {
	err := v.Bcache.CreateBcache(dev, cacheDev, block, bucket)
	if err != nil {
		log.Errorf("create bcache failed device %s cache device %s error %s", dev, cacheDev, err.Error())
		return nil, err
	}
	err = v.Bcache.RegisterDevice(dev, cacheDev)
	if err != nil {
		log.Errorf("register bcache failed device %s cache device %s error %s", dev, cacheDev, err.Error())
		return nil, err
	}
	deviceInfo, err := v.Bcache.GetDeviceBcache(dev)
	if err != nil {
		log.Errorf("get bcache device %s error %s", dev, cacheDev, err.Error())
		return nil, err
	}

	err = v.Bcache.SetCacheMode(deviceInfo.Name, cachePolicy)
	if err != nil {
		log.Errorf("set cache mode failed %s %s", deviceInfo.Name, err.Error())
		return nil, err
	}

	return deviceInfo, nil
}

func (v *LocalVolumeImplement) DeleteBcache(dev, cacheDev string) error {

	deviceInfo, err := v.BcacheDeviceInfo(dev)
	if err != nil && !strings.Contains(err.Error(), "exit status 2") {
		log.Errorf("get device info error %s %s", dev, err.Error())
		return err
	}
	if deviceInfo == nil {
		return nil
	}
	err = v.Bcache.RemoveBcache(deviceInfo)

	if err != nil {
		log.Errorf("delete cache device failed %s", err.Error())
		return err
	}
	return nil
}

func (v *LocalVolumeImplement) BcacheDeviceInfo(dev string) (*types.BcacheDeviceInfo, error) {
	bcacheInfo, err := v.Bcache.ShowDevice(dev)
	if err != nil {
		return nil, err
	}
	bcacheInfo.DevicePath = dev

	deviceInfo, err := v.Bcache.GetDeviceBcache(dev)
	if err != nil {
		log.Errorf("get device info error %s %s", dev, err.Error())
		return nil, err
	}
	bcacheInfo.KernelMajor = deviceInfo.KernelMajor
	bcacheInfo.KernelMinor = deviceInfo.KernelMinor
	bcacheInfo.Name = deviceInfo.Name
	bcacheInfo.BcachePath = deviceInfo.BcachePath

	return bcacheInfo, nil
}

func (v *LocalVolumeImplement) GetLv() lvmd.Lvm2 {
	return v.Lv
}
