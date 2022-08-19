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

package v1

import (
	"google.golang.org/grpc/codes"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LogicVolumeSpec defines the desired state of LogicVolume
type LogicVolumeSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	NodeName    string            `json:"nodeName"`
	Size        resource.Quantity `json:"size"`
	DeviceGroup string            `json:"deviceGroup"`
	Pvc         string            `json:"pvc"`
	NameSpace   string            `json:"nameSpace"`
}

// LogicVolumeStatus defines the observed state of LogicVolume
type LogicVolumeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	VolumeID    string             `json:"volumeID,omitempty"`
	Code        codes.Code         `json:"code,omitempty"`
	Message     string             `json:"message,omitempty"`
	CurrentSize *resource.Quantity `json:"currentSize,omitempty"`
	Status      string             `json:"status,omitempty"`
	DeviceMajor uint32             `json:"deviceMajor,omitempty"`
	DeviceMinor uint32             `json:"deviceMinor,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SIZE",type="string",JSONPath=".spec.size"
// +kubebuilder:printcolumn:name="GROUP",type="string",JSONPath=".spec.deviceGroup"
// +kubebuilder:printcolumn:name="NODE",type="string",JSONPath=".spec.nodeName"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.status"
// +kubebuilder:printcolumn:name="NAMESPACE",type="string",priority=1,JSONPath=".spec.nameSpace"
// +kubebuilder:printcolumn:name="PVC",type="string",priority=1,JSONPath=".spec.pvc"
// +kubebuilder:resource:scope=Cluster,shortName=lv

// LogicVolume is the Schema for the logicvolumes API
type LogicVolume struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LogicVolumeSpec   `json:"spec,omitempty"`
	Status LogicVolumeStatus `json:"status,omitempty"`
}

// IsCompatibleWith returns true if the LogicalVolume is compatible.
func (lv *LogicVolume) IsCompatibleWith(lv2 *LogicVolume) bool {
	if lv.Name != lv2.Name {
		return false
	}
	if lv.Spec.Size.Cmp(lv2.Spec.Size) != 0 {
		return false
	}
	return true
}

// +kubebuilder:object:root=true

// LogicVolumeList contains a list of LogicVolume
type LogicVolumeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LogicVolume `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LogicVolume{}, &LogicVolumeList{})
}
