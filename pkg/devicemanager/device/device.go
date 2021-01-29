package device

import (
	"carina/pkg/devicemanager/types"
	"carina/utils/exec"
	"carina/utils/log"
	"fmt"
	"strings"
)

type LocalDevice interface {
	// ListDevices list all devices available on a machine
	ListDevices() ([]string, error)

	ListDevicesDetail() ([]*types.LocalDisk, error)
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
func (ld *LocalDeviceImplement) ListDevicesDetail() ([]*types.LocalDisk, error) {
	args := []string{"-all", "-noheadings", "--bytes", "--json", "--output", "NAME,FSTYPE,MOUNTPOINT,SIZE,STATE,TYPE,ROTA,RO"}
	devices, err := ld.Executor.ExecuteCommandWithOutput("lsblk", args...)
	if err != nil {
		log.Error("exec lsblk failed" + err.Error())
		return nil, err
	}
	// TODO: 实现解析方法
	parseKeyValuePairString(devices)

	return nil, nil
}

/*
# df /dev/sda
文件系统         1K-块  已用    可用 已用% 挂载点
udev           8193452     0 8193452    0% /dev
*/
func (ld *LocalDeviceImplement) GetDiskUsed(device string) (uint64, error) {
	use, err := ld.Executor.ExecuteCommandWithOutput("df", device)
	if err != nil {
		log.Error("exec df failed" + err.Error())
	}
	// TODO: 实现解析方法
	parseKeyValuePairString(use)
	return 0, nil
}

// converts a raw key value pair string into a map of key value pairs
// example raw string of `foo="0" bar="1" baz="biz"` is returned as:
// map[string]string{"foo":"0", "bar":"1", "baz":"biz"}
func parseKeyValuePairString(propsRaw string) map[string]string {
	// first split the single raw string on spaces and initialize a map of
	// a length equal to the number of pairs
	props := strings.Split(propsRaw, " ")
	propMap := make(map[string]string, len(props))

	for _, kvpRaw := range props {
		// split each individual key value pair on the equals sign
		kvp := strings.Split(kvpRaw, "=")
		if len(kvp) == 2 {
			// first element is the final key, second element is the final value
			// (don't forget to remove surrounding quotes from the value)
			propMap[kvp[0]] = strings.Replace(kvp[1], `"`, "", -1)
		}
	}

	return propMap
}
