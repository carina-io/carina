/*
   Copyright @ 2021 bocloud <fushaosong@beyondcent.com>.

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

const (
	KEYWORD = "carina-"

	// DiskType is a disk type
	DiskType = "disk"
	// SSDType is an sdd type
	SSDType = "ssd"
	// PartType is a partition type
	PartType = "part"
	// CryptType is an encrypted type
	CryptType = "crypt"
	// LVMType is an LVM type
	LVMType    = "lvm"
	RomType    = "rom"
	Lvm2FsType = "LVM2_member"
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
	// Device number
	DeviceNumber string `json:"deviceNumber"`
	// Have partitions
	HavePartitions bool `json:"havePartitions"`
}
