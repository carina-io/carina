package api

// VgGroup defines the observed state of NodeStorageResourceStatus
type VgGroup struct {
	VGName  string    `json:"vgName,omitempty"`
	PVCount uint64    `json:"pvCount,omitempty"`
	LVCount uint64    `json:"lvCount,omitempty"`
	VGAttr  string    `json:"vgAttr,omitempty"`
	VGSize  uint64    `json:"vgSize,omitempty"`
	VGFree  uint64    `json:"vgFree,omitempty"`
	PVS     []*PVInfo `json:"pvs,omitempty"`
}

// PVInfo defines pv details
type PVInfo struct {
	PVName string `json:"pvName,omitempty"`
	VGName string `json:"vgName,omitempty"`
	PVFmt  string `json:"pvFmt,omitempty"`
	PVAttr string `json:"pvAttr,omitempty"`
	PVSize uint64 `json:"pvSize,omitempty"`
	PVFree uint64 `json:"pvFree,omitempty"`
}

// Disk defines disk details
type Disk struct {
	// Name is the kernel name of the disk.
	Name string `json:"name,omitempty"`

	// Path is the device path of the disk.
	Path string `json:"path,omitempty"`

	// Size is the size of the disk in bytes.
	Size uint64 `json:"size"`

	// SectorSize is the sector size of the device, if its unknown or not
	// applicable it will return 0.
	SectorSize uint `json:"sectorSize,omitempty"`

	// ReadOnly - cannot be written to.
	ReadOnly bool `json:"read-only,omitempty"`

	// Type is the DiskType indicating the type of this disk. This value
	// can be used to determine if the disk is of a particular media type like
	// HDD, SSD or NVMe.
	Type DiskType `json:"type,omitempty"`

	// Attachment is the type of storage card this disk is attached to.
	// For example: RAID, ATA or PCIE.
	Attachment AttachmentType `json:"attachment,omitempty"`

	// Partitions is the set of partitions on this disk.
	Partitions PartitionSet `json:"partitions,omitempty"`

	// TableType is the type of the table
	Table TableType `json:"table,omitempty"`

	// Properties are a set of properties of this disk.
	Properties PropertySet `json:"properties,omitempty"`

	// UdevInfo is the disk's udev information.
	UdevInfo UdevInfo `json:"udevInfo,omitempty"`
}
type DiskType int
type AttachmentType int
type TableType int
type Property string

const (
	// Ephemeral - A cloud ephemeral disk.
	Ephemeral Property = "EPHEMERAL"
)

// PropertySet - a group of properties of a disk
type PropertySet map[Property]bool

type PartitionSet map[string]Partition
type GUID []byte
type PartType GUID

// Partition wraps the disk partition information.
type Partition struct {
	// Start is the offset in bytes of the start of this partition.
	Start uint64 `json:"start,omitempty"`

	// Last is the last byte that is part of this partition.
	Last uint64 `json:"last,omitempty"`

	// ID is the partition id.
	ID GUID `json:"id,omitempty"`

	// Type is the partition type.
	Type PartType `json:"type,omitempty"`

	// Name is the name of this partition.
	Name string `json:"name,omitempty"`

	// Number is the number of this partition.
	Number uint `json:"number,omitempty"`
}

type UdevInfo struct {
	// Name of the disk
	Name string `json:"name,omitempty"`

	// SysPath is the system path of this device.
	SysPath string `json:"sysPath,omitempty"`

	// Symlinks for the disk.
	Symlinks []string `json:"symLinks,omitempty"`

	// Properties is udev information as a map of key, value pairs.
	Properties map[string]string `json:"properties,omitempty"`
}

// Raid defines raid details
type Raid struct {
}
