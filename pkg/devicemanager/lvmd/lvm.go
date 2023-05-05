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

package lvmd

import (
	"errors"
	"fmt"
	"github.com/carina-io/carina/api"
	"strings"
	"time"

	"github.com/carina-io/carina/pkg/devicemanager/types"
	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/exec"
	"github.com/carina-io/carina/utils/log"
)

type Lvm2Implement struct {
	Executor exec.Executor
}

func (lv2 *Lvm2Implement) PVCheck(dev string) (string, error) {
	return lv2.Executor.ExecuteCommandWithCombinedOutput("pvck", dev)
}

func (lv2 *Lvm2Implement) PVCreate(dev string) error {
	return lv2.Executor.ExecuteCommand("pvcreate", dev)
}

func (lv2 *Lvm2Implement) PVRemove(dev string) error {
	if err := lv2.Executor.ExecuteCommand("pvremove", dev); err != nil {
		return err
	}
	/*if err := lv2.PVWipefs(dev); err != nil {
		return err
	}*/
	return nil
}

func (lv2 *Lvm2Implement) PVResize(dev string) error {
	return lv2.Executor.ExecuteCommand("pvresize", dev)
}

func (lv2 *Lvm2Implement) PVWipefs(dev string) error {
	return lv2.Executor.ExecuteCommand("wipefs -a", dev)
}

// PVS 示例输出
// pvs --noheadings --separator=, --units=b --nosuffix --unbuffered --nameprefixes
// LVM2_PV_NAME='/dev/loop2',LVM2_VG_NAME='lvmvg',LVM2_PV_FMT='lvm2',LVM2_PV_ATTR='a--',LVM2_PV_SIZE='16101933056',LVM2_PV_FREE='16101933056'
func (lv2 *Lvm2Implement) PVS() ([]api.PVInfo, error) {

	args := []string{"--noheadings", "--separator=,", "--units=b", "--nosuffix", "--unbuffered", "--nameprefixes"}

	pvsInfo, err := lv2.Executor.ExecuteCommandWithOutput("pvs", args...)
	if err != nil {
		return nil, err
	}
	return parsePvs(pvsInfo), nil
}

// PVDisplay
/*
# pvdisplay /dev/loop4
  --- Physical volume ---
  PV Name               /dev/loop4
  VG Name               v1
  PV Size               15.00 GiB / not usable 4.00 MiB
  Allocatable           yes
  PE Size               4.00 MiB
  Total PE              3839
  Free PE               3839
  Allocated PE          0
  PV UUID               OiNoxD-Y1sw-FSzi-mqPN-07EW-C77P-TNdtc6
*/
func (lv2 *Lvm2Implement) PVDisplay(dev string) (*api.PVInfo, error) {
	pvsInfo, err := lv2.PVS()
	if err != nil {
		return nil, err
	}
	for _, pv := range pvsInfo {
		if pv.PVName == dev {
			return &pv, nil
		}
	}
	return nil, errors.New("disk not found")
}

// PVScan runs the `pvscan --cache <dev>` command. It scans for the
// device at `dev` and adds it to the LVM metadata cache if `lvmetad`
// is running. If `dev` is an empty string, it scans all devices.
func (lv2 *Lvm2Implement) PVScan(dev string) error {
	args := []string{"--cache"}
	if dev != "" {
		args = append(args, dev)
	}
	return lv2.Executor.ExecuteCommand("pvscan", args...)
}

func (lv2 *Lvm2Implement) VGCheck(vg string) error {
	return lv2.Executor.ExecuteCommand("vgck", vg)
}

// VGCreate vgcreate --add-tag=v1 v1 /dev/loop4
func (lv2 *Lvm2Implement) VGCreate(vg string, tags, pvs []string) error {
	var args []string
	for _, tag := range tags {
		if tag != "" {
			args = append(args, "--add-tag="+tag)
		}
	}
	args = append(args, vg)
	for _, pv := range pvs {
		args = append(args, pv)
	}
	err := lv2.Executor.ExecuteCommand("vgcreate", args...)
	if err != nil {
		return err
	}

	return nil
}

func (lv2 *Lvm2Implement) VGRemove(vg string) error {
	return lv2.Executor.ExecuteCommand("vgremove", "-f", vg)
}

// VGS 示例
// vgs --noheadings --separator=, --units=b --nosuffix --unbuffered --nameprefixes
// LVM2_VG_NAME='lvmvg',LVM2_PV_COUNT='1',LVM2_LV_COUNT='0',LVM2_SNAP_COUNT='0',LVM2_VG_ATTR='wz--n-',LVM2_VG_SIZE='16101933056',LVM2_VG_FREE='16101933056'
// LVM2_VG_NAME='v1',LVM2_PV_COUNT='2',LVM2_LV_COUNT='0',LVM2_SNAP_COUNT='0',LVM2_VG_ATTR='wz--n-',LVM2_VG_SIZE='32203866112',LVM2_VG_FREE='32203866112'
func (lv2 *Lvm2Implement) VGS() ([]api.VgGroup, error) {
	flieds := []string{"-o", "VG_NAME,PV_COUNT,LV_COUNT,VG_ATTR,VG_SIZE,VG_FREE"}
	args := []string{"--noheadings", "--separator=,", "--units=b", "--nosuffix", "--unbuffered", "--nameprefixes"}

	vgsInfo, err := lv2.Executor.ExecuteCommandWithOutput("vgs", append(flieds, args...)...)
	if err != nil {
		return nil, err
	}

	return parseVgs(vgsInfo), nil
}

func (lv2 *Lvm2Implement) VGDisplay(vg string) (*api.VgGroup, error) {
	vgsInfo, err := lv2.VGS()
	if err != nil {
		return nil, err
	}
	for _, vgs := range vgsInfo {
		if vgs.VGName == vg {
			return &vgs, nil
		}
	}

	return nil, errors.New("vg not found")
}

// VGScan runs the `vgscan --cache <name>` command. It scans for the
// volume group and adds it to the LVM metadata cache if `lvmetad`
// is running. If `name` is an empty string, it scans all volume groups.
func (lv2 *Lvm2Implement) VGScan(vg string) error {
	args := []string{"--cache"}
	if vg != "" {
		args = append(args, vg)
	}
	return lv2.Executor.ExecuteCommand("vgscan", args...)
}

func (lv2 *Lvm2Implement) VGExtend(vg, pv string) error {

	err := lv2.Executor.ExecuteCommand("vgextend", vg, pv)
	if err != nil {
		return err
	}
	return nil
}

// VGReduce
/*
# vgs
  VG    #PV #lv2.#SN Attr   VSize  VFree
  lvmvg   1   0   0 wz--n- 15.00g 15.00g
  v1      2   0   0 wz--n- 29.99g 29.99g
# pvs
  PV         VG    Fmt  Attr PSize  PFree
  /dev/loop2 lvmvg lvm2 a--  15.00g 15.00g
  /dev/loop4 v1    lvm2 a--  15.00g 15.00g
  /dev/loop5 v1    lvm2 a--  15.00g 15.00g
# pvmove /dev/loop5
  No data to move for v1
# vgreduce v1 /dev/loop5
  Removed "/dev/loop5" from volume group "v1"
# pvremove /dev/loop5
  Labels on physical volume "/dev/loop5" successfully wiped
# pvs
  PV         VG    Fmt  Attr PSize  PFree
  /dev/loop2 lvmvg lvm2 a--  15.00g 15.00g
  /dev/loop4 v1    lvm2 a--  15.00g 15.00g
*/
func (lv2 *Lvm2Implement) VGReduce(vg, pv string) error {

	output, err := lv2.Executor.ExecuteCommandWithOutput("pvmove", pv)
	if err != nil && !strings.Contains(output, "No data to move") {
		log.Error(output)
		return err
	}

	log.Info("wait 1s to exec vgreduce ")
	time.Sleep(1 * time.Second)
	if err := lv2.Executor.ExecuteCommand("vgreduce", vg, pv); err != nil {
		return err
	}

	if err = lv2.PVRemove(pv); err != nil {
		return err
	}

	return nil
}

// CreateThinPool lvcreate -T v1/t5 --size 2g
func (lv2 *Lvm2Implement) CreateThinPool(lv, vg string, size uint64) error {
	return lv2.Executor.ExecuteCommand("lvcreate", "-T", fmt.Sprintf("%s/%s", vg, lv), "--size", fmt.Sprintf("%vg", size>>30))
}

// ResizeThinPool lvresize -f -L 6g v1/t5
func (lv2 *Lvm2Implement) ResizeThinPool(lv, vg string, size uint64) error {
	return lv2.Executor.ExecuteCommand("lvresize", "-f", "-L", fmt.Sprintf("%vg", size>>30), fmt.Sprintf("%s/%s", vg, lv))
}

// DeleteThinPool lvremove v1/t3
func (lv2 *Lvm2Implement) DeleteThinPool(lv, vg string) error {
	// TODO: 删除pool前，要保证池子内lvm卷和snapshot已经全部删除
	return lv2.LVRemove(lv, vg)
}

func (lv2 *Lvm2Implement) LVCreateFromPool(lv, thin, vg string, size uint64) error {
	return lv2.Executor.ExecuteCommand("lvcreate", "-T", fmt.Sprintf("%s/%s", vg, thin), "-n", lv, "-V", fmt.Sprintf("%vg", size>>30))
}

// LVCreateFromVG LVCreate creates logical volume in this volume group.
// name is a name of creating volume. size is volume size in bytes. volTags is a
// list of tags to add to the volume.
func (lv2 *Lvm2Implement) LVCreateFromVG(lv, vg string, size uint64, tags []string, stripe uint, stripeSize string) error {
	args := []string{"-n", lv, "-L", fmt.Sprintf("%vg", size>>30), "-W", "y", "-y"}
	for _, tag := range tags {
		if tag != "" {
			args = append(args, "--add-tag="+tag)
		}
	}
	if stripe != 0 {
		args = append(args, "-i", fmt.Sprintf("%d", stripe))

		if stripeSize != "" {
			args = append(args, "-I", stripeSize)
		}
	}
	args = append(args, vg)

	return lv2.Executor.ExecuteCommand("lvcreate", args...)
}

func (lv2 *Lvm2Implement) LVRemove(lv, vg string) error {
	return lv2.Executor.ExecuteCommand("lvremove", "-f", fmt.Sprintf("%s/%s", vg, lv))
}

// LVResize lvresize -L 2g v1/m2
func (lv2 *Lvm2Implement) LVResize(lv, vg string, size uint64) error {
	return lv2.Executor.ExecuteCommand("lvresize", "-L", fmt.Sprintf("%vg", size>>30), fmt.Sprintf("%s/%s", vg, lv))
}

// LVDisplay lvdisplay v1/m2
func (lv2 *Lvm2Implement) LVDisplay(lv, vg string) (*types.LvInfo, error) {
	lvInfo, err := lv2.LVS(fmt.Sprintf("%s/%s", vg, lv))
	if err != nil {
		return nil, err
	}
	if len(lvInfo) < 1 {
		return nil, errors.New("not found")
	}
	return &lvInfo[0], nil
}

// LVS
/*
# lvs -o lv_name,lv_path,lv_size,lv_kernel_major,lv_kernel_minor,origin,origin_size,pool_lv,thin_count,lv_tags --noheadings --separator=, --units=b --nosuffix --unbuffered --nameprefixes
  LVM2_LV_NAME='t1',LVM2_LV_PATH='/dev/v1/t1',LVM2_LV_SIZE='1073741824',LVM2_LV_KERNEL_MAJOR='252',LVM2_LV_KERNEL_MINOR='0',LVM2_ORIGIN='',LVM2_ORIGIN_SIZE='',LVM2_POOL_LV='',LVM2_THIN_COUNT='',LVM2_LV_TAGS='t1'
  LVM2_LV_NAME='t5',LVM2_LV_PATH='',LVM2_LV_SIZE='6979321856',LVM2_LV_KERNEL_MAJOR='252',LVM2_LV_KERNEL_MINOR='3',LVM2_ORIGIN='',LVM2_ORIGIN_SIZE='',LVM2_POOL_LV='',LVM2_THIN_COUNT='1',LVM2_LV_TAGS=''
  LVM2_LV_NAME='m2',LVM2_LV_PATH='/dev/v1/m2',LVM2_LV_SIZE='2147483648',LVM2_LV_KERNEL_MAJOR='252',LVM2_LV_KERNEL_MINOR='5',LVM2_ORIGIN='',LVM2_ORIGIN_SIZE='',LVM2_POOL_LV='t5',LVM2_THIN_COUNT='',LVM2_LV_TAGS=''

*/
func (lv2 *Lvm2Implement) LVS(lvName string) ([]types.LvInfo, error) {
	fields := []string{"-o", "lv_name,vg_name,lv_path,lv_size,data_percent,lv_attr,lv_kernel_major,lv_kernel_minor,origin,origin_size,pool_lv,thin_count,lv_tags,lv_active"}
	args := []string{"--noheadings", "--separator=,", "--units=b", "--nosuffix", "--unbuffered", "--nameprefixes"}

	if lvName != "" {
		args = append(args, lvName)
	}

	lvsInfo, err := lv2.Executor.ExecuteCommandWithOutput("lvs", append(fields, args...)...)
	if err != nil && strings.Contains(lvsInfo, "Failed to find logical volume") {
		return []types.LvInfo{}, nil
	}
	if err != nil {
		return nil, errors.New(lvsInfo)
	}
	return parseLvs(lvsInfo), nil
}

// CreateSnapshot lvcreate -s v1/m2 -n snaph-m1 -ay -Ky
func (lv2 *Lvm2Implement) CreateSnapshot(snap, lv, vg string) error {
	// Pool容量时lv卷的三倍，则能创建两个快照
	// TODO: 需要检查pool>lvm卷,若是相等则不支持创建快照操作
	return lv2.Executor.ExecuteCommand("lvcreate", "-s", fmt.Sprintf("%s/%s", vg, lv), "-n", snap, "-ay", "-Ky")
}

// DeleteSnapshot
func (lv2 *Lvm2Implement) DeleteSnapshot(snap, vg string) error {
	return lv2.LVRemove(snap, vg)
}

// RestoreSnapshot 测试
// mkfs -t ext4 /dev/v1/m2
// mount /dev/v1/m2 /mnt && touch /mnt/1 && touch /mnt/2 && ls
// lvcreate -s v1/m2 -n snap-m1 -ay -Ky
// touch /mnt/3 && touch /mnt/4
// umount /mnt
// lvconvert --merge v1/snap-m1
// mount /dev/v1/m2 /mnt && ls /mnt
func (lv2 *Lvm2Implement) RestoreSnapshot(snap, vg string) error {
	// 恢复快照后，此快照将消失
	// TODO: 恢复快照前要umount
	return lv2.Executor.ExecuteCommand("lvconvert", "--merge", fmt.Sprintf("%s/%s", vg, snap))
}

func (lv2 *Lvm2Implement) StartLvm2() error {
	//err := lv2.Executor.ExecuteCommandResidentBinary(3*time.Second, "lvmetad")
	//if err != nil {
	//	return err
	//}
	if !utils.FileExists("/run/lvm/lvmpolld.socket") {
		err := lv2.Executor.ExecuteCommandResidentBinary(3*time.Second, "lvmpolld")
		if err != nil {
			return err
		}
	}
	return nil
}

func (lv2 *Lvm2Implement) RemoveUnknownDevice(vg string) error {
	return lv2.Executor.ExecuteCommand("vgreduce", "--removemissing", vg)
}
