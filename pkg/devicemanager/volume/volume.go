package volume

import (
	"carina/pkg/devicemanager/lvmd"
	"carina/pkg/devicemanager/types"
	"carina/utils/log"
	"carina/utils/mutx"
	"errors"
	"fmt"
	"strings"
)

const VOLUMEMUTEX = "VolumeMutex"

type LocalVolumeImplement struct {
	Lv    lvmd.Lvm2
	Mutex *mutx.GlobalLocks
}

func (v *LocalVolumeImplement) CreateVolume(lvName, vgName string, size, ratio uint64) error {
	// TODO: 检查一下该Volume是否已经存在
	thinName := THIN + lvName
	// 配置pool和volume倍数比例，为了创建快照做准备，快照需要volume同等的存储空间
	sizePool := size * ratio
	// 首先创建thin pool
	if err := v.Lv.CreateThinPool(thinName, vgName, sizePool); err != nil {
		return err
	}
	// 创建volume卷
	name := LVVolume + lvName
	if err := v.Lv.LVCreateFromPool(name, thinName, vgName, size); err != nil {
		return err
	}

	return nil
}

func (v *LocalVolumeImplement) DeleteVolume(lvName, vgName string) error {

	lvInfo, err := v.Lv.LVDisplay(lvName, vgName)
	if err != nil {
		return err
	}
	// TODO: 解析lvInfo 获取thin pool name
	thinName := lvInfo

	if err := v.Lv.LVRemove(lvName, vgName); err != nil {
		return err
	}

	// ToDO: 需要检查pool中是否有快照

	if err := v.Lv.DeleteThinPool(thinName, vgName); err != nil {
		return err
	}

	return nil
}

func (v *LocalVolumeImplement) ResizeVolume(lvName, vgName string, size, ratio uint64) error {

	name := LVVolume + lvName
	_, err := v.Lv.LVDisplay(name, vgName)
	if err != nil {
		return err
	}
	// TODO: 判断volume容量是否已经扩容过

	thinName := THIN + lvName
	sizePool := size * ratio
	if err := v.Lv.ResizeThinPool(thinName, vgName, sizePool); err != nil {
		return err
	}

	if err := v.Lv.LVResize(name, vgName, size); err != nil {
		return err
	}

	return nil
}

func (v *LocalVolumeImplement) VolumeList(lvName, vgName string) error {
	return nil
}

func (v *LocalVolumeImplement) CreateSnapshot(snapName, lvName, vgName string) error {

	name := SNAP + snapName
	if err := v.Lv.CreateSnapshot(name, lvName, vgName); err != nil {
		return err
	}

	return nil
}

func (v *LocalVolumeImplement) DeleteSnapshot(snapName, vgName string) error {
	if err := v.Lv.DeleteSnapshot(snapName, vgName); err != nil {
		return nil
	}
	return nil
}

func (v *LocalVolumeImplement) RestoreSnapshot(snapName, vgName string) error {
	// TODO： 需要检查是否已经umount
	// 恢复快照会导致快照消失
	if err := v.Lv.RestoreSnapshot(snapName, vgName); err != nil {
		return err
	}
	return nil
}

func (v *LocalVolumeImplement) SnapshotList(lvName, vgName string) error {
	return nil
}

func (v *LocalVolumeImplement) CloneVolume(lvName, vgName, newLvName string) error {
	// 获取pool lv，创建一个和一模一样对池子
	_, err := v.Lv.LVDisplay(lvName, vgName)
	if err != nil {
		return err
	}
	// TODO: 获取池子大小
	size := uint64(100)
	// 创建thin pool
	thinName := THIN + lvName
	if err := v.Lv.CreateThinPool(thinName, vgName, size); err != nil {
		return err
	}
	name := LVVolume + lvName
	if err := v.Lv.LVCreateFromPool(name, thinName, vgName, size); err != nil {
		return err
	}

	// TODO： 分两种情况，若是从快照克隆则挂在快照进行文件拷贝，若是克隆volume则创建快照后挂在拷贝

	// mount -t

	return nil
}

func (v *LocalVolumeImplement) GetCurrentVgStruct() ([]types.VgGroup, error) {

	resp := []types.VgGroup{}
	tmp := map[string]*types.VgGroup{}
	vgs, err := v.Lv.VGS()
	if err != nil {
		return nil, err
	}
	for _, v := range vgs {
		tmp[v.VGName] = &v
	}

	pvs, err := v.Lv.PVS()
	if err != nil {
		return nil, err
	}
	for _, v := range pvs {
		if tmp[v.VGName] != nil {
			tmp[v.VGName].PVS = append(tmp[v.VGName].PVS, &v)
		}
	}

	for _, v := range tmp {
		resp = append(resp, *v)
	}

	return resp, nil
}

func (v *LocalVolumeImplement) GetCurrentPvStruct() ([]types.PVInfo, error) {
	return v.Lv.PVS()
}

func (v *LocalVolumeImplement) AddNewDiskToVg(disk, vgName string) error {
	if !v.Mutex.TryAcquire(VOLUMEMUTEX) {
		log.Info("get global mutex failed")
		return nil
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
			return errors.New(fmt.Sprintf("pv %s have bind vg %s ", pvInfo.PVName, pvInfo.VGName))
		}
	}
	// 检查PV,决定新创建还是扩容
	vgInfo, err := v.Lv.VGDisplay(vgName)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		log.Errorf("get vg detail failed %s", err.Error())
		return err
	}
	if vgInfo == nil {
		err := v.Lv.VGCreate(vgName, []string{vgName}, []string{disk})
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

	// 刷新缓存
	if err := v.Lv.PVScan(""); err != nil {
		log.Warnf(" error during pvscan: %v", err)
	}

	if err := v.Lv.VGScan(""); err != nil {
		log.Warnf("error during vgscan: %v", err)
	}
	return nil
}
func (v *LocalVolumeImplement) RemoveDiskInVg(disk, vgName string) error {
	if !v.Mutex.TryAcquire(VOLUMEMUTEX) {
		log.Info("get global mutex failed")
		return nil
	}
	defer v.Mutex.Release(VOLUMEMUTEX)

	// 确保PV存在
	pvInfo, err := v.Lv.PVDisplay(disk)
	if err != nil {
		log.Info("get pv detail failed %s", err.Error())
		return err
	}
	if pvInfo == nil {
		log.Warnf("this pv not found %s", disk)
		return nil
	} else {
		if pvInfo.VGName != vgName {
			log.Errorf("pv %s have bind vg %s not %s ", pvInfo.PVName, pvInfo.VGName, vgName)
			return errors.New(fmt.Sprintf("pv %s have bind vg %s not %s ", pvInfo.PVName, pvInfo.VGName, vgName))
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
	// 检查PV,决定新创建还是扩容
	vgInfo, err := v.Lv.VGDisplay(vgName)
	if err != nil {
		log.Errorf("get vg detail failed %s", err.Error())
		return err
	}
	if vgInfo == nil {
		log.Errorf("vg %s not found", vgName)
		return nil
	} else {
		// 当vg卷下只有一个pv时，需要检查是否还存在lv
		if vgInfo.PVCount == 1 {
			if vgInfo.LVCount > 0 || vgInfo.SnapCount > 0 {
				log.Warnf("cannot remove the disk %s because there are still lv volumes", disk)
				return nil
			}
			err = v.Lv.VGRemove(vgName)
			if err != nil {
				log.Errorf("vg remove failed %s", vgName)
				return err
			}
			err = v.Lv.PVRemove(disk)
			if err != nil {
				log.Errorf("pv remove failed %s", disk)
				return err
			}
		} else {
			err = v.Lv.VGReduce(vgName, disk)
			if err != nil {
				log.Errorf("vgreduce failed %s %s", vgName, disk)
				return err
			}
		}
	}

	// 刷新缓存
	if err := v.Lv.PVScan(""); err != nil {
		log.Warnf(" error during pvscan: %v", err)
	}

	if err := v.Lv.VGScan(""); err != nil {
		log.Warnf("error during vgscan: %v", err)
	}

	return nil
}
