package api

// VgGroup defines the observed state of NodeStorageResourceStatus
type VgGroup struct {
	VGName    string    `json:"vgName,omitempty"`
	PVName    string    `json:"pvName,omitempty"`
	PVCount   uint64    `json:"pvCount,omitempty"`
	LVCount   uint64    `json:"lvCount,omitempty"`
	SnapCount uint64    `json:"snapCount,omitempty"`
	VGAttr    string    `json:"vgAttr,omitempty"`
	VGSize    uint64    `json:"vgSize,omitempty"`
	VGFree    uint64    `json:"vgFree,omitempty"`
	PVS       []*PVInfo `json:"pvs,omitempty"`
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
	ParentName string      `json:"parentName"`
	Capacity   string      `json:"capacity,omitempty"`
	Available  string      `json:"available,omitempty"`
	Partition  []Partition `json:"partition,omitempty"`
	FreeSpace  []Partition `json:"freespace,omitempty"`
}

// Partition defines disk partition details
type Partition struct {
	Number     string `json:"number,omitempty"`
	Start      string `json:"start,omitempty"`
	End        string `json:"end,omitempty"`
	Size       string `json:"size,omitempty"`
	Filesystem string `json:"filesystem,omitempty"`
	Name       string `json:"name,omitempty"`
	Flags      string `json:"flags,omitempty"`
}

// Raid defines raid details
type Raid struct {
}
