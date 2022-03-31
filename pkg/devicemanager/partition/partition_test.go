package partition

import (
	"fmt"
	"sort"
	"testing"

	"github.com/anuvu/disko"
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
	disk, err := mysys.ScanDisk(fname)
	assert.NoError(t, err)
	fmt.Println(disk.UdevInfo.Properties)
	fmt.Println(disk.Name)
	fmt.Println(disk.Partitions)
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
	fname := "/dev/loop3"
	disk, err := mysys.ScanDisk(fname)
	assert.NoError(t, err)
	fmt.Println(disk.Partitions)

}

func TestCreatePartition(t *testing.T) {
	localparttion := NewLocalPartitionImplement()
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
