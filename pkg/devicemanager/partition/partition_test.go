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
	"strings"
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
	t.Log(diskSet)
}
func TestScanDisks(t *testing.T) {

	diskSet, err := mysys.ScanDisks(matchAll, "/dev/loop2")
	assert.NoError(t, err)
	t.Log(diskSet)

}

func TestScanDisk(t *testing.T) {
	fname := "/dev/loop2"
	disk, err := mysys.ScanDisk(fname)
	assert.NoError(t, err)
	t.Log(disk.UdevInfo)
	t.Log(disk)

	//t.Log(disk.FreeSpacesWithMin(5000))
	//t.Log(disk.FreeSpaces()[0].Size(), disk.FreeSpaces()[0].Size()>>30)
	for _, part := range disk.Partitions {
		name := linux.GetPartitionKname(disk.Path, part.Number)
		t.Log(name)
		partinfo, err := linux.GetUdevInfo(name)
		assert.NoError(t, err)
		t.Log(partinfo)
	}

}

func TestGetDiskPartMaxNum(t *testing.T) {
	fname := "/dev/loop3"
	disk, err := mysys.ScanDisk(fname)
	assert.NoError(t, err)

	number := []int{}
	for k := range disk.Partitions {
		number = append(number, int(k))
	}
	sort.Ints(number)
	t.Log(number[0], number[len(number)-1])
	for _, v := range disk.FreeSpaces() {
		t.Log(v.Size())
	}

}

func TestAddPartition(t *testing.T) {
	fname := "/dev/loop2"
	var size uint64 = 50
	disk, err := mysys.ScanDisk(fname)
	assert.NoError(t, err)
	fs := disk.FreeSpacesWithMin(size)
	t.Log(fs)
	var partitionNum uint

	for i := uint(1); i < 128; i++ {
		if _, exists := disk.Partitions[i]; !exists {
			partitionNum = i
			break
		}
	}
	myGUID := disko.GenGUID()
	part := disko.Partition{
		Start:  fs[0].Start,
		Last:   fs[0].Last,
		Type:   partid.LinuxLVM,
		Name:   "test6",
		ID:     myGUID,
		Number: partitionNum,
	}

	err = mysys.CreatePartition(disk, part)
	assert.NoError(t, err)
	disk, err = mysys.ScanDisk(fname)
	assert.NoError(t, err)

	t.Logf("%s\n", disk.Details())
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
	t.Log(name)
	partinfo, err := linux.GetUdevInfo(name)
	assert.NoError(t, err)
	t.Log(partinfo)

}

func TestMount(t *testing.T) {

	targetPathOut, err := localparttion.Executor.ExecuteCommandWithOutput("/usr/bin/findmnt", "-S", "/dev/loop0", "--noheadings", "--output=target")
	t.Log("targetPathOut", targetPathOut)
	if err != nil {
		t.Log(err.Error())
	}

	targetpath := strings.TrimSpace(strings.TrimSuffix(strings.ReplaceAll(targetPathOut, "\"", ""), "\n"))
	flag := strings.Contains(targetpath, "mount")
	t.Log(flag)
	t.Log("len:", len(targetpath))
}

func TestCreatePartition(t *testing.T) {
	//name: 54cd2f39cf95 group: carina-raw-loop size: 13958643712
	size := 4747316223
	lvName := "pvc-58ad162c-1815-476b-9b3d-4735f652842e"
	err := localparttion.CreatePartition(utils.PartitionName(lvName), "carina-raw-loop/loop2", uint64(size))
	assert.NoError(t, err)
	disk, err := mysys.ScanDisk("/dev/loop2")
	assert.NoError(t, err)
	t.Log(disk.Partitions)
}

func TestUpdatePartition(t *testing.T) {
	size := 10747316223
	lvName := "pvc-58ad162c-1815-476b-9b3d-4735f652842e"
	group := "carina-raw-ssd/loop2"
	part, err := localparttion.GetPartition(utils.PartitionName(lvName), group)
	t.Log("number", part.Number, " name:", part.Name, " size ", part.Size())
	assert.NoError(t, err)
	err = localparttion.UpdatePartition(utils.PartitionName(lvName), group, uint64(size))
	assert.NoError(t, err)
	disk, err := mysys.ScanDisk("/dev/loop2")
	assert.NoError(t, err)
	for _, v := range disk.Partitions {
		t.Log(v.Name, v.Number, v.Start, v.Last)
	}

}
func TestDeletePartitionByNumber(t *testing.T) {
	disk, err := mysys.ScanDisk("/dev/loop2")
	assert.NoError(t, err)
	for _, v := range disk.Partitions {
		err := localparttion.DeletePartitionByPartNumber(disk, v.Number)
		assert.NoError(t, err)
	}

	disk, err = mysys.ScanDisk("/dev/loop4")
	assert.NoError(t, err)
	t.Log(disk)

}

func TestDeletePartition(t *testing.T) {
	lvName := "pvc-58ad162c-1815-476b-9b3d-4735f652842e"
	err := localparttion.DeletePartition(utils.PartitionName(lvName), "carina-raw-loop/loop2")
	assert.NoError(t, err)
	_, err = mysys.ScanDisk("/dev/loop2")
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

func TestClearPartition(t *testing.T) {

	disklist, err := localparttion.ListDevicesDetail("")
	if err != nil {
		t.Errorf("run command losetup  fail %s", err)
	}

	for _, d := range disklist {
		disk, err := mysys.ScanDisk(d.Name)
		if err != nil {
			t.Errorf("get disk info error %s", err.Error())

		}
		fmt.Println(disk.Partitions)
		t.Logf("disk path: %s ,parttions len %d", d.Name, len(disk.Partitions))
		if len(disk.Partitions) < 1 {
			continue
		}
		for _, p := range disk.Partitions {
			t.Logf("parttions %s", p.Name)
			if !strings.Contains(p.Name, "carina.io") {
				continue
			}
			t.Logf("remove parttions %s %d %d", p.Name, p.Start, p.Last)
			if err := localparttion.DeletePartitionByPartNumber(disk, p.Number); err != nil {
				t.Errorf("delete parttions in  %s device %d error %s", disk.Name, p.Number, err.Error())
			}

		}

	}

}
