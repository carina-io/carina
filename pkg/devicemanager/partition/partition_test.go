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
	"fmt"
	"sort"
	"testing"

	"github.com/anuvu/disko"
	"github.com/anuvu/disko/linux"
	"github.com/anuvu/disko/partid"
	"github.com/carina-io/carina/utils"
	"github.com/stretchr/testify/assert"
)

var localparttion = NewLocalPartitionImplement()

func TestScanAllDisk(t *testing.T) {
	diskSet, err := mysys.ScanAllDisks(matchAll)
	assert.NoError(t, err)
	fmt.Println(diskSet)
}
func TestScanDisks(t *testing.T) {

	diskSet, err := mysys.ScanDisks(matchAll, "/dev/loop2")
	assert.NoError(t, err)
	fmt.Println(diskSet)
}

func TestScanDisk(t *testing.T) {
	fname := "/dev/loop3"
	//kname := linux.GetPartitionKname("/dev/loop3p1", 1)
	//fmt.Println(kname)
	//devadminfo, err := linux.GetUdevInfo(kname)
	//assert.NoError(t, err)
	//fmt.Println(devadminfo)
	disk, err := mysys.ScanDisk(fname)
	assert.NoError(t, err)
	fmt.Println(disk.UdevInfo.Properties)
	fmt.Println(disk.Name)

	fmt.Println(disk.FreeSpacesWithMin(5000))

}

func TestGetDiskPartMaxNum(t *testing.T) {
	fname := "/dev/loop2"
	disk, err := mysys.ScanDisk(fname)
	assert.NoError(t, err)

	number := []int{}
	for k, _ := range disk.Partitions {
		number = append(number, int(k))
	}
	sort.Ints(number)
	fmt.Println(number[0], number[len(number)-1])
	for _, v := range disk.FreeSpaces() {
		fmt.Println(v.Size())
	}

}

func TestAddPartition(t *testing.T) {
	fname := "/dev/loop2"
	var size uint64 = 50
	disk, err := mysys.ScanDisk(fname)
	assert.NoError(t, err)
	fs := disk.FreeSpacesWithMin(size)
	fmt.Println(fs)
	number := []int{}
	for k, _ := range disk.Partitions {
		number = append(number, int(k))
	}
	sort.Ints(number)
	myGUID := disko.GenGUID()
	part := disko.Partition{
		Start:  fs[0].Start,
		Last:   fs[0].Last,
		Type:   partid.LinuxLVM,
		Name:   "test6",
		ID:     myGUID,
		Number: uint(number[len(number)-1]) + 1,
	}

	err = mysys.CreatePartition(disk, part)
	assert.NoError(t, err)
	disk, err = mysys.ScanDisk(fname)
	assert.NoError(t, err)

	fmt.Printf("%s\n", disk.Details())
	assert.NoError(t, err)

}

func TestGetPartitions(t *testing.T) {
	lvName := "pvc-e11aa51b-2c8f-4de2-bee7-73a1af0a0c36"
	group := "csi-carina-raw/loop3"
	part, err := localparttion.GetPartition(utils.PartitionName(lvName), group)
	assert.NoError(t, err)
	disk, err := localparttion.ScanDisk(group)
	assert.NoError(t, err)
	name := linux.GetPartitionKname(disk.Path, part.Number)
	fmt.Println(name)
	partinfo, err := linux.GetUdevInfo(name)
	assert.NoError(t, err)
	fmt.Println(partinfo)

}

func TestCreatePartition(t *testing.T) {
	//name: 54cd2f39cf95 group: carina-raw-loop size: 13958643712
	size := 4747316223
	lvName := "pvc-58ad162c-1815-476b-9b3d-4735f652842e"
	err := localparttion.CreatePartition(utils.PartitionName(lvName), "carina-raw-loop/loop3", uint64(size))
	assert.NoError(t, err)
	disk, err := mysys.ScanDisk("/dev/loop3")
	assert.NoError(t, err)
	fmt.Println(disk.Partitions)
}

func TestUpdatePartition(t *testing.T) {
	size := 5747316223
	lvName := "pvc-58ad162c-1815-476b-9b3d-4735f652842e"
	group := "carina-raw-loop/loop3"
	err := localparttion.UpdatePartition(utils.PartitionName(lvName), group, uint64(size))
	assert.NoError(t, err)
	_, err = mysys.ScanDisk("/dev/loop3")
	assert.NoError(t, err)

}

func TestDeletePartition(t *testing.T) {
	lvName := "pvc-58ad162c-1815-476b-9b3d-4735f652842e"
	err := localparttion.DeletePartition(utils.PartitionName(lvName), "carina-raw-loop/loop3")
	assert.NoError(t, err)
	_, err = mysys.ScanDisk("/dev/loop3")
	assert.NoError(t, err)

}

func TestAddDiskLoop(t *testing.T) {

	args := []string{"--size=100G", "/tmp/disk.device"}
	_, err := localparttion.Executor.ExecuteCommandWithOutput("truncate", args...)
	if err != nil {
		t.Errorf("run command truncate -size=100G /tmp/disk.device fail %s", err)
	}
	args = []string{"-f", "/tmp/disk.device"}
	_, err = localparttion.Executor.ExecuteCommandWithOutput("losetup", args...)
	if err != nil {
		t.Errorf("run command losetup  fail %s", err)
	}
	_, err = mysys.ScanDisk("/dev/loop1")
	assert.NoError(t, err)
}

func TestDelDiskLoop(t *testing.T) {

	args := []string{"-d", "/dev/loop2"}
	_, err := localparttion.Executor.ExecuteCommandWithOutput("losetup", args...)
	if err != nil {
		t.Errorf("run command losetup  fail %s", err)
	}

}
