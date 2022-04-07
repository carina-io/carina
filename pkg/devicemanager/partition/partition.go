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

package partition

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/anuvu/disko"
	"github.com/anuvu/disko/linux"
	"github.com/anuvu/disko/partid"
	"github.com/carina-io/carina/pkg/devicemanager/types"
	"github.com/carina-io/carina/utils/exec"
	"github.com/carina-io/carina/utils/log"
	"github.com/carina-io/carina/utils/mutx"
)

var (
	matchAll = func(d disko.Disk) bool {
		return true
	}
	mysys = linux.System()
)

type LocalPartition interface {
	ScanAllDisks(filter disko.DiskFilter) (disko.DiskSet, error)
	ScanAllDisk(paths []string) (disko.DiskSet, error)
	ScanDisk(groups string) (disko.Disk, error)
	CreatePartition(name, groups string, size uint64) error
	GetPartition(name, groups string) (disko.Partition, error)
	UpdatePartition(name, groups string, size uint64) error
	DeletePartition(name, groups string) error
	DeletePartitionByPartNumber(disk disko.Disk, number uint) error
	UpdatePartitionCache(name string, number uint) error
	Wipe(name, groups string) error
	UdevSettle() error
	ListDevicesDetail(device string) ([]*types.LocalDisk, error)
}

const DISKMUTEX = "DiskMutex"

type LocalPartitionImplement struct {
	//	Bcache               bcache.Bcache
	Mutex            *mutx.GlobalLocks
	CacheParttionNum map[string]uint
	Executor         exec.Executor
}

func NewLocalPartitionImplement() *LocalPartitionImplement {
	executor := &exec.CommandExecutor{}
	mutex := mutx.NewGlobalLocks()
	return &LocalPartitionImplement{
		Mutex:            mutex,
		CacheParttionNum: make(map[string]uint),
		Executor:         executor}
}
func (ld *LocalPartitionImplement) ListDevicesDetail(device string) ([]*types.LocalDisk, error) {
	args := []string{"--pairs", "--paths", "--bytes", "--output", "NAME,FSTYPE,MOUNTPOINT,SIZE,STATE,TYPE,ROTA,RO,PKNAME"}
	if device != "" {
		args = append(args, device)
	}
	devices, err := ld.Executor.ExecuteCommandWithOutput("lsblk", args...)
	if err != nil {
		log.Error("exec lsblk failed" + err.Error())
		return nil, err
	}

	return filter(parseDiskString(devices)), nil
}

func parseDiskString(diskString string) []*types.LocalDisk {
	resp := []*types.LocalDisk{}

	if diskString == "" {
		return resp
	}

	diskString = strings.ReplaceAll(diskString, "\"", "")
	//diskString = strings.ReplaceAll(diskString, " ", "")

	vgsList := strings.Split(diskString, "\n")
	for _, vgs := range vgsList {
		tmp := types.LocalDisk{}
		vg := strings.Split(vgs, " ")
		for _, v := range vg {
			k := strings.Split(v, "=")

			switch k[0] {
			case "NAME":
				tmp.Name = k[1]
			case "MOUNTPOINT":
				tmp.MountPoint = k[1]
			case "SIZE":
				tmp.Size, _ = strconv.ParseUint(k[1], 10, 64)
			case "STATE":
				tmp.State = k[1]
			case "TYPE":
				tmp.Type = k[1]
			case "ROTA":
				tmp.Rotational = k[1]
			case "RO":
				if k[1] == "1" {
					tmp.Readonly = true
				} else {
					tmp.Readonly = false
				}
			case "FSTYPE":
				tmp.Filesystem = k[1]
			case "PKNAME":
				tmp.ParentName = k[1]
			default:
				log.Warnf("undefined filed %s-%s", k[0], k[1])
			}
		}

		resp = append(resp, &tmp)
	}
	return resp

}

func filter(disklist []*types.LocalDisk) (DiskList []*types.LocalDisk) {
	for _, d := range disklist {
		if d.Type == "part" || d.ParentName != "" {
			continue
		}
		if strings.Contains(d.Name, types.KEYWORD) {
			continue
		}

		if d.Readonly || d.Size < 10<<30 || d.Filesystem != "" || d.MountPoint != "" {
			//log.Infof("mismatched disk: %s filesystem:%s mountpoint:%s readonly:%t, size:%d", d.Name, d.Filesystem, d.MountPoint, d.Readonly, d.Size)
			continue
		}
		DiskList = append(DiskList, d)
	}
	return DiskList
}

func (ld *LocalPartitionImplement) ScanAllDisk(paths []string) (disko.DiskSet, error) {
	matchAll = func(d disko.Disk) bool {
		return true
	}
	diskSet, err := mysys.ScanDisks(matchAll, paths...)
	if err != nil {
		log.Errorf("scan  node disk resource error %s", err.Error())
		return disko.DiskSet{}, err
	}
	return diskSet, nil
}

func (ld *LocalPartitionImplement) ScanAllDisks(filter disko.DiskFilter) (disko.DiskSet, error) {
	diskSet, err := mysys.ScanAllDisks(filter)
	if err != nil {
		log.Errorf("scan  node disk resource error %s", err.Error())
		return disko.DiskSet{}, err
	}
	return diskSet, nil
}

func (ld *LocalPartitionImplement) ScanDisk(groups string) (disko.Disk, error) {
	//selectDeviceGroup := strings.Split(groups, "-")[1]

	diskPath := strings.Split(groups, "/")[1]
	return mysys.ScanDisk(fmt.Sprintf("/dev/%s", diskPath))
}

func (ld *LocalPartitionImplement) GetPartition(name, groups string) (disko.Partition, error) {
	diskPath := strings.Split(groups, "/")[1]
	disk, err := ld.ScanDisk(groups)
	if err != nil {
		log.Error("scanDisk path ", fmt.Sprintf("/dev/%s", diskPath), "failed"+err.Error())
		return disko.Partition{}, err
	}
	if len(disk.Partitions) < 1 {
		return disko.Partition{}, nil
	}
	partitionName := name
	for _, part := range disk.Partitions {
		if part.Name == partitionName {
			return part, nil
		}
	}
	return disko.Partition{}, nil

}

func parseUdevInfo(output string) map[string]string {
	lines := strings.Split(output, "\n")
	result := make(map[string]string, len(lines))
	for _, v := range lines {
		pairs := strings.Split(v, "=")
		if len(pairs) > 1 {
			result[pairs[0]] = pairs[1]
		}
	}
	return result
}
func (ld *LocalPartitionImplement) CreatePartition(name, groups string, size uint64) error {
	//DeviceGroup=deviceGroup + "/" + device.Name
	partitionName := name
	if _, ok := ld.CacheParttionNum[partitionName]; ok {
		return nil
	}
	diskPath := strings.Split(groups, "/")[1]
	log.Info("create parttion: group:", groups, "path:", diskPath, "size", size)
	if !ld.Mutex.TryAcquire(DISKMUTEX) {
		log.Info("wait other task release mutex, please retry...")
		return errors.New("get global mutex failed")
	}
	defer ld.Mutex.Release(DISKMUTEX)

	disk, err := ld.ScanDisk(groups)
	if err != nil {
		log.Error("scanDisk path ", fmt.Sprintf("/dev/%s", diskPath), "failed"+err.Error())
		return err
	}

	fs := disk.FreeSpacesWithMin(size)
	if len(fs) < 1 {
		log.Error("path ", fmt.Sprintf("/dev/%s", diskPath), "has not free size "+err.Error())
		return err
	}
	var partitionNum uint

	for i := uint(1); i < 128; i++ {
		if _, exists := disk.Partitions[i]; !exists {
			partitionNum = i
			break
		}
	}

	if partitionNum == 0 {
		return fmt.Errorf("failed to find an open partition number %d", partitionNum)
	}

	last := fs[0].Last
	if (last - fs[0].Start) > uint64(size) {
		last = fs[0].Start + uint64(size) - 1
	}

	part := disko.Partition{
		Start:  fs[0].Start,
		Last:   last,
		Type:   partid.LinuxFS,
		Name:   partitionName,
		Number: partitionNum,
	}
	log.Info("create parttion", part)
	err = mysys.CreatePartition(disk, part)
	if err != nil {
		log.Error("create parttion on disk ", fmt.Sprintf("/dev/%s", diskPath), "failed"+err.Error())
		return err
	}
	ld.CacheParttionNum[partitionName] = partitionNum
	log.Info("create parttion success", partitionNum, ld.CacheParttionNum)
	return nil
}

func (ld *LocalPartitionImplement) UpdatePartition(name, groups string, size uint64) error {

	if !ld.Mutex.TryAcquire(DISKMUTEX) {
		log.Info("wait other task release mutex, please retry...")
		return errors.New("get global mutex failed")
	}
	defer ld.Mutex.Release(DISKMUTEX)
	//selectDeviceGroup := strings.Split(groups, "-")[1]
	diskPath := strings.Split(groups, "/")[1]

	disk, err := ld.ScanDisk(groups)
	if err != nil {
		log.Error("scanDisk path ", fmt.Sprintf("/dev/%s", diskPath), "failed"+err.Error())
		return err
	}

	fs := disk.FreeSpacesWithMin(size)
	if len(fs) < 1 {
		log.Error("path ", fmt.Sprintf("/dev/%s", diskPath), "has not free size ")
		return errors.New("disk has not free size" + fmt.Sprintf("/dev/%s", diskPath))
	}
	if len(disk.Partitions) < 1 {
		log.Error("path", fmt.Sprintf("/dev/%s", diskPath), "disk has mutipod used")
		return errors.New("disk has mutipod used" + fmt.Sprintf("/dev/%s", diskPath))
	}
	// partitionName := name
	// log.Info("path", fmt.Sprintf("/dev/%s", diskPath), "cacheParttionMap:", ld.CacheParttionNum)
	// if _, ok := ld.CacheParttionNum[partitionName]; !ok {
	// 	log.Error("path", fmt.Sprintf("/dev/%s", diskPath), " cacheParttionMap has no parttion number")
	// 	//return errors.New("cacheParttionMap has no parttion number" + fmt.Sprintf("/dev/%s", diskPath))
	// }
	var partitionNum uint
	for _, p := range disk.Partitions {
		log.Info(p)
		if p.Name != name {
			continue
		}
		log.Info("Update parttion on disk src: ", fmt.Sprintf("/dev/%s", diskPath), " number:", p.Number, " name:", p.Name, " start:", p.Start, "size: ", p.Size(), " latest:", p.Last)
		p.Last = size
		partitionNum = p.Number
		log.Info("Update parttion on disk dst: ", fmt.Sprintf("/dev/%s", diskPath), " number:", p.Number, " name:", p.Name, " start:", p.Start, "size: ", size, " latest:", p.Last)
		if err := mysys.UpdatePartition(disk, p); err != nil {
			log.Error("Update parttion on disk ", fmt.Sprintf("/dev/%s", diskPath), "failed"+err.Error())
			return errors.New("Update parttion on disk failed" + fmt.Sprintf("/dev/%s", diskPath))
		}
	}
	ld.CacheParttionNum[name] = partitionNum
	log.Info("update parttion success", partitionNum, ld.CacheParttionNum)

	return nil
}

func (ld *LocalPartitionImplement) DeletePartition(name, groups string) error {
	if !ld.Mutex.TryAcquire(DISKMUTEX) {
		log.Info("wait other task release mutex, please retry...")
		return errors.New("get global mutex failed")
	}
	defer ld.Mutex.Release(DISKMUTEX)

	//selectDeviceGroup := strings.Split(groups, "-")[1]
	diskPath := strings.Split(groups, "/")[1]
	disk, err := mysys.ScanDisk(fmt.Sprintf("/dev/%s", diskPath))
	if err != nil {
		log.Error("scanDisk path ", fmt.Sprintf("/dev/%s", diskPath), "failed"+err.Error())
		return err
	}
	log.Info("delete parttion: group:", groups, " path:", diskPath, " name ", name, " cacheParttionNum ", ld.CacheParttionNum)
	//var partitionNum uint
	// partitionName := name
	if _, ok := ld.CacheParttionNum[name]; !ok {
		log.Error("path", fmt.Sprintf("/dev/%s", diskPath), "cacheParttionMap has no parttion number")
		//return errors.New("cacheParttionMap has no parttion number" + fmt.Sprintf("/dev/%s", diskPath))
	}
	// number := ld.CacheParttionNum[partitionName]
	// if _, ok := disk.Partitions[number]; !ok {
	// 	return fmt.Errorf("partition %d does not exist", number)
	// }
	for _, p := range disk.Partitions {
		if p.Name != name {
			continue
		}
		//partitionNum = p.Number
		log.Info("Delete parttion on disk:", disk, " number:", p.Number)
		if err := mysys.DeletePartition(disk, p.Number); err != nil {
			log.Error("Delete parttion on disk ", fmt.Sprintf("/dev/%s", diskPath), "failed"+err.Error())
			return errors.New("Delete parttion on disk failed" + fmt.Sprintf("/dev/%s", diskPath))
		}

	}
	delete(ld.CacheParttionNum, name)

	return nil

}
func (ld *LocalPartitionImplement) DeletePartitionByPartNumber(disk disko.Disk, number uint) error {
	if !ld.Mutex.TryAcquire(DISKMUTEX) {
		log.Info("wait other task release mutex, please retry...")
		return errors.New("get global mutex failed")
	}
	defer ld.Mutex.Release(DISKMUTEX)
	log.Info("clean parttion on disk by number:", disk, " number:", number)
	if err := mysys.DeletePartition(disk, number); err != nil {
		log.Error("Delete parttion on disk ", disk.Path, "failed "+err.Error())
		return errors.New("Delete parttion on disk failed" + disk.Path)
	}
	for k, v := range ld.CacheParttionNum {
		if v == number {
			delete(ld.CacheParttionNum, k)
		}
	}

	return nil

}

func (ld *LocalPartitionImplement) UpdatePartitionCache(name string, number uint) error {
	log.Info("update CacheParttionNum success", number, ld.CacheParttionNum)
	if _, ok := ld.CacheParttionNum[name]; !ok {
		ld.CacheParttionNum[name] = number
		log.Info("update CacheParttionNum success", number, ld.CacheParttionNum)
	}
	return nil

}

func (ld *LocalPartitionImplement) Wipe(name, groups string) error {
	if !ld.Mutex.TryAcquire(DISKMUTEX) {
		log.Info("wait other task release mutex, please retry...")
		return errors.New("get global mutex failed")
	}
	defer ld.Mutex.Release(DISKMUTEX)

	selectDeviceGroup := strings.Split(groups, "-")[1]
	diskPath := strings.Split(selectDeviceGroup, "/")[1]
	disk, err := mysys.ScanDisk(fmt.Sprintf("/dev/%s", diskPath))
	if err != nil {
		log.Error("scanDisk path ", fmt.Sprintf("/dev/%s", diskPath), "failed"+err.Error())
		return err
	}
	if err := mysys.Wipe(disk); err != nil {
		return err
	}
	return ld.UdevSettle()
}
func (ld *LocalPartitionImplement) UdevSettle() error {
	_, err := ld.Executor.ExecuteCommandWithOutput("udevadm", "settle")
	if err != nil {
		return err
	}
	return err
}
