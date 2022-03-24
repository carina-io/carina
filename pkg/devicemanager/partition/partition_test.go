package partition

import (
	"fmt"
	"testing"

	"github.com/carina-io/carina/pkg/devicemanager/device"
	"github.com/carina-io/carina/utils/exec"
	"github.com/stretchr/testify/assert"
)

var (
	executor  = &exec.CommandExecutor{}
	deviceImp = &device.LocalDeviceImplement{Executor: executor}
	parttion  = &LocalPartitionImplement{LocalDeviceImplement: *deviceImp, Executor: executor}
)

func TestAddPartition(t *testing.T) {
	parttion, err := parttion.AddPartition("/dev/loop2", "test", "2M", "10M")
	assert.NoError(t, err)
	fmt.Println(parttion)
}
func TestGetPartitions(t *testing.T) {
	parttion, err := parttion.GetPartitions("/dev/loop2")
	assert.NoError(t, err)
	fmt.Println(parttion)
}

func TestListDiskPartitions(t *testing.T) {
	disks, err := parttion.ListDiskPartitions()
	if err != nil {
		return
	}
	//assert.NoError(t, err)
	fmt.Println(disks)
}

func TestListPartitions(t *testing.T) {
	partitions, err := parttion.ListPartitions()
	if err != nil {
		return
	}
	fmt.Println(partitions)
}

func TestGetDiskInfo(t *testing.T) {
	disk, err := parttion.GetDiskInfo("loop1")
	if err != nil {
		return
	}
	//assert.NoError(t, err)
	fmt.Println(disk)
}

func TestGetUdevInfo(t *testing.T) {
	disk, err := parttion.GetUdevInfo("loop1")
	if err != nil {
		return
	}
	assert.NoError(t, err)
	fmt.Println(disk)
}
