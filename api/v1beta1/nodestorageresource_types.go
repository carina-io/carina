/*
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

package v1beta1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NodeStorageResourceSpec defines the desired state of NodeStorageResource
type NodeStorageResourceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of NodeStorageResource. Edit nodestorageresource_types.go to remove/update
	NodeName string `json:"nodeName,omitempty"`
}

// NodeStorageResourceStatus defines the observed state of NodeStorageResource
type NodeStorageResourceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	SyncTime time.Time `json:"syncTime,omitempty"`
	// Capacity represents the total resources of a node.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#capacity
	// +optional
	Capacity map[string]resource.Quantity `json:"capacity,omitempty"`
	// Allocatable represents the resources of a node that are available for scheduling.
	// Defaults to Capacity.
	// +optional
	Allocatable map[string]resource.Quantity `json:"available,omitempty"`
	// +optional
	VgGroups []VgGroup `json:"vgGroups,omitempty"`
	// +optional
	Disks []Disk `json:"disks,,omitempty"`
	// +optional
	RAIDs []Raid `json:"raids,omitempty"`
}

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

type Raid struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=nsr
// +kubebuilder:printcolumn:name="node",type="string",JSONPath=".spec.nodeName"
// +kubebuilder:printcolumn:name="time",type="string",JSONPath=".status.syncTime"

// NodeStorageResource is the Schema for the nodestorageresources API
type NodeStorageResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeStorageResourceSpec   `json:"spec,omitempty"`
	Status NodeStorageResourceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NodeStorageResourceList contains a list of NodeStorageResource
type NodeStorageResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeStorageResource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeStorageResource{}, &NodeStorageResourceList{})
}
