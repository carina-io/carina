package types

// vg卷组信息及映射关系
type VgGroup struct {
	VGName    string    `json:"vgName"`
	PVCount   uint64    `json:"pvCount"`
	SnapCount uint64    `json:"snapCount"`
	VGAttr    string    `json:"vgAttr"`
	VGSize    uint64    `json:"vgSize"`
	VGFree    uint64    `json:"vgFree"`
	PVS       []*PVInfo `json:"pvs"`
}

// pv详细信息
type PVInfo struct {
	PVName string `json:"pvName"`
	VGName string `json:"vgName"`
	PVFmt  string `json:"pvFmt"`
	PVAttr string `json:"pvAttr"`
	PVSize uint64 `json:"pvSize"`
	PVFree string `json:"pvFree"`
}

// lv详细信息
type LvInfo struct {
	LVName        string `json:"lvName"`
	VGName        string `json:"vgName"`
	LVPath        string `json:"lvPath"`
	LVSize        uint64 `json:"lvSize"`
	LVKernelMajor uint64 `json:"lvKernelMajor"`
	LVKernelMinor uint64 `json:"lvKernelMinor"`
	PoolLV        string `json:"poolLv"`
	ThinCount     uint64 `json:"thinCount"`
	LVTags        string `json:"lvTags"`
}
