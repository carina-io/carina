package types


type LocalDisk struct {
	// Name is the device name
	Name string `json:"name"`
	// Parent is the device parent's name
	Parent string `json:"parent"`
	// HasChildren is whether the device has a children device
	HasChildren bool `json:"hasChildren"`
	// DevLinks is the persistent device path on the host
	DevLinks string `json:"devLinks"`
	// Size is the device capacity in byte
	Size uint64 `json:"size"`
	// UUID is used by /dev/disk/by-uuid
	UUID string `json:"uuid"`
	// Serial is the disk serial used by /dev/disk/by-id
	Serial string `json:"serial"`
	// Type is disk type
	Type string `json:"type"`
	// Rotational is the boolean whether the device is rotational: true for hdd, false for ssd and nvme
	Rotational bool `json:"rotational"`
	// ReadOnly is the boolean whether the device is readonly
	Readonly bool `json:"readOnly"`
	// Partitions is a partition slice
	Partitions []Partition
	// Filesystem is the filesystem currently on the device
	Filesystem string `json:"filesystem"`
	// Vendor is the device vendor
	Vendor string `json:"vendor"`
	// Model is the device model
	Model string `json:"model"`
	// WWN is the world wide name of the device
	WWN string `json:"wwn"`
	// WWNVendorExtension is the WWN_VENDOR_EXTENSION from udev info
	WWNVendorExtension string `json:"wwnVendorExtension"`
	// Empty checks whether the device is completely empty
	Empty bool `json:"empty"`
	// Information provided by Ceph Volume Inventory
	CephVolumeData string `json:"cephVolumeData,omitempty"`
	// RealPath is the device pathname behind the PVC, behind /mnt/<pvc>/name
	RealPath string `json:"real-path,omitempty"`
	// KernelName is the kernel name of the device
	KernelName string `json:"kernel-name,omitempty"`
	// Whether this device should be encrypted
	Encrypted bool `json:"encrypted,omitempty"`
}

type Partition struct {
	Name       string
	Size       uint64
	Label      string
	Filesystem string
}
