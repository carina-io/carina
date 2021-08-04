/*
   Copyright @ 2021 fushaosong <fushaosong@beyondlet.com>.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/
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
	LVName        string  `json:"lvName"`
	VGName        string  `json:"vgName"`
	LVPath        string  `json:"lvPath"`
	LVSize        uint64  `json:"lvSize"`
	LVKernelMajor uint32  `json:"lvKernelMajor"`
	LVKernelMinor uint32  `json:"lvKernelMinor"`
	Origin        string  `json:"origin"`
	OriginSize    uint64  `json:"originSize"`
	PoolLV        string  `json:"poolLv"`
	ThinCount     uint64  `json:"thinCount"`
	LVTags        string  `json:"lvTags"`
	DataPercent   float64 `json:"dataPercent"`
	LVAttr        string  `json:"lvAttr"`
	LVActive      string  `json:"lvActive"`
}
