/*
Copyright 2021.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FilePermissionsSpec defines the desired state of FilePermissions
type FilePermissionsSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	PvRefUID       types.UID `json:"pvrefuid" protobuf:"bytes,5,opt,name=PvRef,casttype=k8s.io/kubernetes/pkg/types.UID"`
	PvcRefUID      types.UID `json:"pvcrefuid" protobuf:"bytes,5,opt,name=PvcRef,casttype=k8s.io/kubernetes/pkg/types.UID"`
	PvcName        string    `json:"pvcname" description:"Name of PersistentVolumeClaim"`
	PvcNamespace   string    `json:"pvcnamespace" description:"Namespace of PersistentVolumeClaim"`
	PermissionsSet bool      `json:"permissionsset" description:"Flag"`
}

type FilePermissionsType struct{}
type ConditionStatus struct{}

// FilePermissionsStatus defines the observed state of FilePermissions
type FilePermissionsStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Type   FilePermissionsType `json:"type" description:"type of Foo condition"`
	Status ConditionStatus     `json:"status" description:"status of the condition, one of True, False, Unknown"`

	// +optional
	Reason *string `json:"reason,omitempty" description:"one-word CamelCase reason for the condition's last transition"`
	// +optional
	Message *string `json:"message,omitempty" description:"human-readable message indicating details about last transition"`

	// +optional
	LastHeartbeatTime *metav1.Time `json:"lastHeartbeatTime,omitempty" description:"last time we got an update on a given condition"`
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty" description:"last time the condition transit from one status to another"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster,shortName=fp,singular=filepermission

// FilePermissions is the Schema for the filepermissions API
type FilePermissions struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FilePermissionsSpec   `json:"spec,omitempty"`
	Status FilePermissionsStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FilePermissionsList contains a list of FilePermissions
type FilePermissionsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FilePermissions `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FilePermissions{}, &FilePermissionsList{})
}
