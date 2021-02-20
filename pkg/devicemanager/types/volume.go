package types

// vg卷组信息及映射关系
type VgGroup struct {
	VGName    string    `json:"vgName"`
	PVName    string    `json:"pvName"`
	PVCount   uint64    `json:"pvCount"`
	LVCount   uint64    `json:"lvCount"`
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
	PVFree uint64 `json:"pvFree"`
}

// lv详细信息
type LvInfo struct {
	LVName        string `json:"lvName"`
	VGName        string `json:"vgName"`
	LVPath        string `json:"lvPath"`
	LVSize        uint64 `json:"lvSize"`
	LVKernelMajor uint32 `json:"lvKernelMajor"`
	LVKernelMinor uint32 `json:"lvKernelMinor"`
	Origin        string `json:"origin"`
	OriginSize    uint64 `json:"originSize"`
	PoolLV        string `json:"poolLv"`
	ThinCount     uint64 `json:"thinCount"`
	LVTags        string `json:"lvTags"`
	DataPercent   string `json:"dataPercent"`
	LVAttr        string `json:"lvAttr"`
	LVActive      string `json:"lvActive"`
}
