package deviceManager

import (
	"carina/utils/exec"
	"carina/utils/log"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
)

var (
	isRBD = regexp.MustCompile("^rbd[0-9]+p?[0-9]{0,}$")
)

func supportedDeviceType(device string) bool {
	return device == DiskType ||
		device == SSDType ||
		device == CryptType ||
		device == LVMType ||
		device == MultiPath ||
		device == PartType ||
		device == LinearType
}

// GetDeviceEmpty check whether a device is completely empty
func GetDeviceEmpty(device *LocalDisk) bool {
	return device.Parent == "" && supportedDeviceType(device.Type) && len(device.Partitions) == 0 && device.Filesystem == ""
}

func ignoreDevice(d string) bool {
	return isRBD.MatchString(d)
}

// DiscoverDevices returns all the details of devices available on the local node
func DiscoverDevices(executor exec.Executor) ([]*LocalDisk, error) {
	var disks []*LocalDisk
	devices, err := ListDevices(executor)
	if err != nil {
		return nil, err
	}

	for _, d := range devices {
		// Ignore RBD device
		if ignoreDevice(d) {
			// skip device
			log.Warnf("skipping rbd device %q", d)
			continue
		}

		// Populate device information coming from lsblk
		disk, err := PopulateDeviceInfo(d, executor)
		if err != nil {
			log.Warnf("skipping device %q. %v", d, err)
			continue
		}

		// Populate udev information coming from udev
		disk, err = PopulateDeviceUdevInfo(d, executor, disk)
		if err != nil {
			// go on without udev info
			// not ideal for our filesystem check later but we can't really fail either...
			log.Warnf("failed to get udev info for device %q. %v", d, err)
		}

		// Test if device has child, if so we skip it and only consider the partitions
		// which will come in later iterations of the loop
		// We only test if the type is 'disk', this is a property reported by lsblk
		// and means it's a parent block device
		if disk.Type == DiskType {
			deviceChild, err := ListDevicesChild(executor, d)
			if err != nil {
				log.Warnf("failed to detect child devices for device %q, assuming they are none. %v", d, err)
			}
			// lsblk will output at least 2 lines if they are partitions, one for the parent
			// and N for the child
			if len(deviceChild) > 1 {
				log.Infof("skipping device %q because it has child, considering the child instead.", d)
				continue
			}
		}

		disks = append(disks, disk)
	}
	log.Debugf("discovered disks are %v", disks)

	return disks, nil
}

// PopulateDeviceInfo returns the information of the specified block device
func PopulateDeviceInfo(d string, executor exec.Executor) (*LocalDisk, error) {
	diskProps, err := GetDeviceProperties(d, executor)
	if err != nil {
		return nil, err
	}

	diskType, ok := diskProps["TYPE"]
	if !ok {
		return nil, errors.New("diskType is empty")
	}
	if !supportedDeviceType(diskType) {
		return nil, fmt.Errorf("unsupported diskType %+s", diskType)
	}

	// get the UUID for disks
	var diskUUID string
	if diskType != PartType {
		diskUUID, err = GetDiskUUID(d, executor)
		if err != nil {
			return nil, err
		}
	}

	disk := &LocalDisk{Name: d, UUID: diskUUID}

	if val, ok := diskProps["TYPE"]; ok {
		disk.Type = val
	}
	if val, ok := diskProps["SIZE"]; ok {
		if size, err := strconv.ParseUint(val, 10, 64); err == nil {
			disk.Size = size
		}
	}
	if val, ok := diskProps["ROTA"]; ok {
		if rotates, err := strconv.ParseBool(val); err == nil {
			disk.Rotational = rotates
		}
	}
	if val, ok := diskProps["RO"]; ok {
		if ro, err := strconv.ParseBool(val); err == nil {
			disk.Readonly = ro
		}
	}
	if val, ok := diskProps["PKNAME"]; ok {
		if val != "" {
			disk.Parent = path.Base(val)
		}
	}
	if val, ok := diskProps["NAME"]; ok {
		disk.RealPath = val
	}
	if val, ok := diskProps["KNAME"]; ok {
		disk.KernelName = path.Base(val)
	}

	return disk, nil
}

// PopulateDeviceUdevInfo fills the udev info into the block device information
func PopulateDeviceUdevInfo(d string, executor exec.Executor, disk *LocalDisk) (*LocalDisk, error) {
	udevInfo, err := GetUdevInfo(d, executor)
	if err != nil {
		return disk, err
	}
	// parse udev info output
	if val, ok := udevInfo["DEVLINKS"]; ok {
		disk.DevLinks = val
	}
	if val, ok := udevInfo["ID_FS_TYPE"]; ok {
		disk.Filesystem = val
	}
	if val, ok := udevInfo["ID_SERIAL"]; ok {
		disk.Serial = val
	}

	if val, ok := udevInfo["ID_VENDOR"]; ok {
		disk.Vendor = val
	}

	if val, ok := udevInfo["ID_MODEL"]; ok {
		disk.Model = val
	}

	if val, ok := udevInfo["ID_WWN_WITH_EXTENSION"]; ok {
		disk.WWNVendorExtension = val
	}

	if val, ok := udevInfo["ID_WWN"]; ok {
		disk.WWN = val
	}

	return disk, nil
}
