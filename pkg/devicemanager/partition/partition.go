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
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/anuvu/disko"
	"github.com/anuvu/disko/linux"
	"github.com/anuvu/disko/partid"
	"github.com/carina-io/carina/utils/exec"
	"github.com/carina-io/carina/utils/log"
	"github.com/carina-io/carina/utils/mutx"
)

var (
	matchAll = func(d disko.Disk) bool {
		return true
	}
	xenbusSysPathMatch = regexp.MustCompile(`/dev/sd-\d+/block/`)

	matchRe = func(d disko.Disk) bool {
		if xenbusSysPathMatch.MatchString(d.UdevInfo.SysPath) {
			return true
		}
		return false
	}
	mysys = linux.System()
)

type LocalPartition interface {
	ScanAllDisks(filter disko.DiskFilter) (disko.DiskSet, error)
	ScanDisk(groups string) (disko.Disk, error)
	CreatePartition(name, groups string, size uint64) error
	UpdatePartition(name, groups string, size uint64) error
	DeletePartition(name, groups string) error
	DeletePartitionByPartNumber(disk disko.Disk, number uint)
	Wipe(name, groups string) error
	UdevSettle() error
}

const DISKMUTEX = "DiskMutex"

type LocalPartitionImplement struct {
	//	LocalDeviceImplement device.LocalDeviceImplement
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

func (ld *LocalPartitionImplement) ScanAllDisks(filter disko.DiskFilter) (disko.DiskSet, error) {
	diskSet, err := mysys.ScanAllDisks(filter)
	if err != nil {
		log.Errorf("scan  node disk resource error %s", err.Error())
		return disko.DiskSet{}, err
	}
	return diskSet, nil
}

func (ld *LocalPartitionImplement) ScanDisk(groups string) (disko.Disk, error) {
	selectDeviceGroup := strings.Split(groups, "-")[1]
	diskPath := strings.Split(selectDeviceGroup, "/")[1]
	disk, err := mysys.ScanDisk(fmt.Sprintf("/dev/%s", diskPath))
	if err != nil {
		log.Error("scanDisk path ", fmt.Sprintf("/dev/%s", diskPath), "failed"+err.Error())
		return disko.Disk{}, err
	}
	return disk, nil

}
func (ld *LocalPartitionImplement) CreatePartition(name, groups string, size uint64) error {
	//DeviceGroup=node.Name + "-" + deviceGroup + "/" + device.Name + "/" + start,

	selectDeviceGroup := strings.Split(groups, "-")[1]
	diskPath := strings.Split(selectDeviceGroup, "/")[1]

	startString := strings.Split(selectDeviceGroup, "/")[2]
	start, err := strconv.ParseInt(startString, 10, 64)
	if err != nil {
		log.Error(" transfer startString ", "failed"+err.Error())
		return err
	}

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
	//check start location is in fs
	var inFreeSizeSpace bool = false
	for _, f := range fs {
		if f.Start == uint64(start) {
			inFreeSizeSpace = true
		}
	}
	if !inFreeSizeSpace {
		start = int64(fs[0].Start)
	}

	number := []int{}
	for k, _ := range disk.Partitions {
		number = append(number, int(k))
	}
	sort.Ints(number)
	myGUID := disko.GenGUID()
	partitionNum := uint(number[len(number)-1]) + 1
	part := disko.Partition{
		Start:  uint64(start),
		Last:   uint64(size),
		Type:   partid.LinuxLVM,
		Name:   name,
		ID:     myGUID,
		Number: partitionNum,
	}

	err = mysys.CreatePartition(disk, part)
	if err != nil {
		log.Error("create parttion on disk ", fmt.Sprintf("/dev/%s", diskPath), "failed"+err.Error())
		return err
	}
	ld.CacheParttionNum[name] = partitionNum
	return nil
}

func (ld *LocalPartitionImplement) UpdatePartition(name, groups string, size uint64) error {

	if !ld.Mutex.TryAcquire(DISKMUTEX) {
		log.Info("wait other task release mutex, please retry...")
		return errors.New("get global mutex failed")
	}
	defer ld.Mutex.Release(DISKMUTEX)
	selectDeviceGroup := strings.Split(groups, "-")[1]
	diskPath := strings.Split(selectDeviceGroup, "/")[1]

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
	if _, ok := ld.CacheParttionNum[name]; !ok {
		log.Error("path", fmt.Sprintf("/dev/%s", diskPath), "cacheParttionMap has no parttion number")
		return errors.New("cacheParttionMap has no parttion number" + fmt.Sprintf("/dev/%s", diskPath))
	}
	if p, ok := disk.Partitions[ld.CacheParttionNum[name]]; ok {
		p.Last = size
		if err := mysys.UpdatePartition(disk, p); err != nil {
			log.Error("Update parttion on disk ", fmt.Sprintf("/dev/%s", diskPath), "failed"+err.Error())
			return errors.New("Update parttion on disk failed" + fmt.Sprintf("/dev/%s", diskPath))
		}
	}

	return nil
}

func (ld *LocalPartitionImplement) DeletePartition(name, groups string) error {
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
	if _, ok := ld.CacheParttionNum[name]; !ok {
		log.Error("path", fmt.Sprintf("/dev/%s", diskPath), "cacheParttionMap has no parttion number")
		return errors.New("cacheParttionMap has no parttion number" + fmt.Sprintf("/dev/%s", diskPath))
	}
	number := ld.CacheParttionNum[name]
	if _, ok := disk.Partitions[number]; !ok {
		return fmt.Errorf("partition %d does not exist", number)
	}
	if err := mysys.DeletePartition(disk, number); err != nil {
		log.Error("Delete parttion on disk ", fmt.Sprintf("/dev/%s", diskPath), "failed"+err.Error())
		return errors.New("Delete parttion on disk failed" + fmt.Sprintf("/dev/%s", diskPath))
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
	if err := mysys.DeletePartition(disk, number); err != nil {
		log.Error("Delete parttion on disk ", disk.Path, "failed"+err.Error())
		return errors.New("Delete parttion on disk failed" + disk.Path)
	}
	for k, v := range ld.CacheParttionNum {
		if v == number {
			delete(ld.CacheParttionNum, k)
		}
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
