package types

const (
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
	// LinearType is a linear type
	LinearType = "linear"
	SgdiskCmd  = "sgdisk"
	// CephLVPrefix is the prefix of a LV owned by ceph-volume
	CephLVPrefix = "ceph--"
	// DeviceMapperPrefix is the prefix of a LV from the device mapper interface
	DeviceMapperPrefix = "dm-"
)
