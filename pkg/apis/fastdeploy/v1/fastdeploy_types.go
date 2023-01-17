/*
Copyright 2023 mark.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FastDeploySpec defines the desired state of FastDeploy
type FastDeploySpec struct {
	// application name
	// If not specified, a random name will be used
	// +kubebuilder:validation:MaxLength:=50
	Name string `json:"name,omitempty"`

	// application namespace
	// If not specified, a random name will be used
	// +kubebuilder:validation:MaxLength:=64
	Namespace string `json:"namespace,omitempty"`

	// container image
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type:int
	Image string `json:"image,omitempty"`

	// application port
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type:int
	Port int32 `json:"port"`

	// application replicas
	// +kubebuilder:validation:Minimum:=1
	Replicas *int32 `json:"replicas,omitempty"`

	// whether to create service
	EnableService bool `json:"enableService"`
}

// FastDeployStatus defines the observed state of FastDeploy
type FastDeployStatus struct {
	// application running status
	Status string `json:"status"`

	// service ip
	ServiceIP string `json:"serviceIp"`
}

//+genclient
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=fd

// FastDeploy is the Schema for the fastdeploys API
type FastDeploy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FastDeploySpec   `json:"spec,omitempty"`
	Status FastDeployStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FastDeployList contains a list of FastDeploy
type FastDeployList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FastDeploy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FastDeploy{}, &FastDeployList{})
}
