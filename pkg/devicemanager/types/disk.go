package types

const (
	KEYWORD = "carina-"
	VGSSD   = "carina-vg-ssd"
	VGHDD   = "carina-vg-hdd"

	// DiskType is a disk type
	DiskType = "disk"
	// SSDType is an sdd type
	SSDType = "ssd"
	// PartType is a partition type
	PartType = "part"
	// CryptType is an encrypted type
	CryptType = "crypt"
	// LVMType is an LVM type
	LVMType = "lvm"
	// MultiPath is for multipath devices
	MultiPath = "mpath"
)

type LocalDisk struct {
	// Name is the device name
	Name string `json:"name"`
	// mount point
	MountPoint string `json:"mountPoint"`
	// Size is the device capacity in byte
	Size uint64 `json:"size"`
	// status
	State string `json:"state"`
	// Type is disk type
	Type string `json:"type"`
	// 1 for hdd, 0 for ssd and nvme
	Rotational string `json:"rotational"`
	// ReadOnly is the boolean whether the device is readonly
	Readonly bool `json:"readOnly"`
	// Filesystem is the filesystem currently on the device
	Filesystem string `json:"filesystem"`
	// has used
	Used uint64 `json:"used"`
	// parent Name
	ParentName string `json:"parentName"`
}
