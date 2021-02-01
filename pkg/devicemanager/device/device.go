package device

import (
	"carina/pkg/devicemanager/types"
	"carina/utils/exec"
	"carina/utils/log"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

type LocalDevice interface {
	// ListDevices list all devices available on a machine
	ListDevices() ([]string, error)

	ListDevicesDetail(device string) ([]*types.LocalDisk, error)
	GetDiskUsed(device string) (uint64, error)
}

type LocalDeviceImplement struct {
	Executor exec.Executor
}

func (ld *LocalDeviceImplement) ListDevices() ([]string, error) {
	devices, err := ld.Executor.ExecuteCommandWithOutput("lsblk", "--all", "--noheadings", "--list", "--output", "KNAME")
	if err != nil {
		return nil, fmt.Errorf("failed to list all devices: %+v", err)
	}

	return strings.Split(devices, "\n"), nil
}

/*
# lsblk -all --bytes --json --output NAME,FSTYPE,MOUNTPOINT,SIZE,STATE,TYPE,ROTA,RO
{
   "blockdevices": [
      {"name": "sda", "fstype": null, "mountpoint": null, "size": "85899345920", "state": "running", "type": "disk", "rota": "1", "ro": "0"},
      {"name": "sda1", "fstype": "ext4", "mountpoint": "/", "size": "81604378624", "state": null, "type": "part", "rota": "1", "ro": "0"},
      {"name": "sda2", "fstype": null, "mountpoint": null, "size": "1024", "state": null, "type": "part", "rota": "1", "ro": "0"},
      {"name": "sda5", "fstype": "swap", "mountpoint": "[SWAP]", "size": "4291821568", "state": null, "type": "part", "rota": "1", "ro": "0"},
      {"name": "sdb", "fstype": null, "mountpoint": null, "size": "87926702080", "state": "running", "type": "disk", "rota": "1", "ro": "0"},
      {"name": "sr0", "fstype": "iso9660", "mountpoint": "/media/ubuntu/VBox_GAs_6.1.16", "size": "60987392", "state": "running", "type": "rom", "rota": "1", "ro": "0"},
      {"name": "loop0", "fstype": "squashfs", "mountpoint": "/snap/core/10583", "size": "102637568", "state": null, "type": "loop", "rota": "1", "ro": "1"},
      {"name": "loop1", "fstype": "squashfs", "mountpoint": "/snap/core/9289", "size": "101724160", "state": null, "type": "loop", "rota": "1", "ro": "1"},
      {"name": "loop2", "fstype": "LVM2_member", "mountpoint": null, "size": "16106127360", "state": null, "type": "loop", "rota": "1", "ro": "0"},
      {"name": "loop3", "fstype": "LVM2_member", "mountpoint": null, "size": "16106127360", "state": null, "type": "loop", "rota": "1", "ro": "0"},
      {"name": "v1-t1", "fstype": null, "mountpoint": null, "size": "1073741824", "state": "running", "type": "lvm", "rota": "1", "ro": "0"},
      {"name": "v1-t5_tmeta", "fstype": null, "mountpoint": null, "size": "4194304", "state": "running", "type": "lvm", "rota": "1", "ro": "0"},
      {"name": "v1-t5-tpool", "fstype": null, "mountpoint": null, "size": "6979321856", "state": "running", "type": "lvm", "rota": "1", "ro": "0"},
      {"name": "v1-t5", "fstype": null, "mountpoint": null, "size": "6979321856", "state": "running", "type": "lvm", "rota": "1", "ro": "0"},
      {"name": "v1-m2", "fstype": "ext4", "mountpoint": null, "size": "2147483648", "state": "running", "type": "lvm", "rota": "1", "ro": "0"},
      {"name": "v1-t5_tdata", "fstype": null, "mountpoint": null, "size": "6979321856", "state": "running", "type": "lvm", "rota": "1", "ro": "0"},
      {"name": "v1-t5-tpool", "fstype": null, "mountpoint": null, "size": "6979321856", "state": "running", "type": "lvm", "rota": "1", "ro": "0"},
      {"name": "v1-t5", "fstype": null, "mountpoint": null, "size": "6979321856", "state": "running", "type": "lvm", "rota": "1", "ro": "0"},
      {"name": "v1-m2", "fstype": "ext4", "mountpoint": null, "size": "2147483648", "state": "running", "type": "lvm", "rota": "1", "ro": "0"},
      {"name": "loop4", "fstype": null, "mountpoint": null, "size": "16106127360", "state": null, "type": "loop", "rota": "1", "ro": "0"},
      {"name": "loop5", "fstype": null, "mountpoint": null, "size": null, "state": null, "type": "loop", "rota": "1", "ro": "0"},
      {"name": "loop6", "fstype": null, "mountpoint": null, "size": null, "state": null, "type": "loop", "rota": "1", "ro": "0"},
      {"name": "loop7", "fstype": null, "mountpoint": null, "size": null, "state": null, "type": "loop", "rota": "1", "ro": "0"}
   ]
}
*/
func (ld *LocalDeviceImplement) ListDevicesDetail(device string) ([]*types.LocalDisk, error) {
	args := []string{"-all", "-noheadings", "--bytes", "--json", "--output", "NAME,FSTYPE,MOUNTPOINT,SIZE,STATE,TYPE,ROTA,RO"}
	if device != "" {
		args = append(args, device)
	}
	devices, err := ld.Executor.ExecuteCommandWithOutput("lsblk", args...)
	if err != nil {
		log.Error("exec lsblk failed" + err.Error())
		return nil, err
	}

	return parseDiskString(devices), nil
}

/*
# df /dev/sda
文件系统         1K-块  已用    可用 已用% 挂载点
udev           8193452     0 8193452    0% /dev
*/
func (ld *LocalDeviceImplement) GetDiskUsed(device string) (uint64, error) {
	_, err := os.Stat(device)
	if err != nil {
		return 1, err
	}
	var stat syscall.Statfs_t
	syscall.Statfs(device, &stat)
	return stat.Blocks - stat.Bavail, nil
}

func parseDiskString(diskString string) []*types.LocalDisk {
	resp := []*types.LocalDisk{}
	type device struct {
		Blockdevices []struct {
			Name       string `json:"name"`
			Fstype     string `json:"fstype"`
			MountPoint string `json:"mountpoint"`
			Size       string `json:"size"`
			State      string `json:"state"`
			Type       string `json:"type"`
			Rota       string `json:"rota"`
			RO         string `json:"ro"`
		} `json:"blockdevices"`
	}
	disk := device{}
	err := json.Unmarshal([]byte(diskString), &disk)
	if err != nil {
		log.Errorf("disk serialize failed %s", err.Error())
		return resp
	}
	for _, ld := range disk.Blockdevices {
		tmp := types.LocalDisk{
			Name:       "/dev/" + ld.Name,
			MountPoint: ld.MountPoint,
			State:      ld.State,
			Type:       ld.Type,
			Rotational: ld.Rota,
			Filesystem: ld.Fstype,
			Used:       0,
		}

		tmp.Size, _ = strconv.ParseUint(ld.Size, 10, 64)
		if ld.RO == "1" {
			tmp.Readonly = true
		}
		resp = append(resp, &tmp)
	}
	return resp
}
