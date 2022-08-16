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
	"github.com/carina-io/carina/api"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	SyncTime metav1.Time `json:"syncTime,omitempty"`
	// Capacity represents the total resources of a node.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#capacity
	// +optional
	Capacity map[string]resource.Quantity `json:"capacity,omitempty"`
	// Allocatable represents the resources of a node that are available for scheduling.
	// Defaults to Capacity.
	// +optional
	Allocatable map[string]resource.Quantity `json:"allocatable,omitempty"`
	// +optional
	VgGroups []api.VgGroup `json:"vgGroups,omitempty"`
	// +optional
	Disks []api.Disk `json:"disks,,omitempty"`
	// +optional
	RAIDs []api.Raid `json:"raids,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="node",type="string",JSONPath=".spec.nodeName"
// +kubebuilder:printcolumn:name="time",type="date",JSONPath=".status.syncTime"
// +kubebuilder:resource:scope=Cluster,shortName=nsr

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
