package partition

import (
	"fmt"
	"sort"
	"testing"

	"github.com/anuvu/disko"
	"github.com/anuvu/disko/partid"
	"github.com/stretchr/testify/assert"
)

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
	fname := "/dev/loop2"
	disk, err := mysys.ScanDisk(fname)
	assert.NoError(t, err)
	fmt.Println(disk.UdevInfo.Properties)
	fmt.Println(disk.Name)
	fmt.Println(disk.Partitions)

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
	fname := "/dev/loop2"
	disk, err := mysys.ScanDisk(fname)
	assert.NoError(t, err)
	fmt.Println(disk.Partitions)
}
