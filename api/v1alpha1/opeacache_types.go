/*
Copyright 2025.

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
	"fmt"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type HuggingFaceModel struct {

	// +kubebuilder:validation:MinLength=1

	// RepoID model unique id
	RepoID string `json:"repoId"`
	// +optional
	RepoType string `json:"repoType,omitempty"`

	// +optional
	FileNames []string `json:"fileNames,omitempty"`

	// +optional
	Revision string `json:"revision,omitempty"`

	// +kubebuilder:validation:MinLength=1

	// Token is the API token for the HuggingFaceModelCli model hub
	Token string `json:"token"`
}

type ModelHub struct {
	From   string      `json:"from"`
	Params []v1.EnvVar `json:"params"`
}

type ModelStorage struct {
	PVC PVC `json:"pvc"`
}
type PVC struct {
	SubPath                      string `json:"subPath"`
	Create                       *bool  `json:"create,omitempty"`
	v1.PersistentVolumeClaimSpec `json:",inline"`
}

type JobConf struct {
	// +optional
	Resources v1.ResourceRequirements `json:"resources,omitempty"`

	// +optional
	Env []v1.EnvVar `json:"env,omitempty"`
}

// OpeaCacheSpec defines the desired state of OpeaCache.
type OpeaCacheSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Download GenAI models from the specified registry
	Sources []ModelHub `json:"sources"`
	// +optional
	ShardedParams []v1.EnvVar `json:"shardedParams,omitempty"`
	// Cache the downloaded models in the specified storage
	Storage ModelStorage `json:"storage"`
	// Job configuration for the model/data download
	JobConf JobConf `json:"jobConf,omitempty"`
}

// OpeaCachePhase describes the phase of the OpeaCache
// +kubebuilder:validation:Enum=Pending;Executing;Ready;Failed
type OpeaCachePhase string

const (
	OpeaCachePhasePending    OpeaCachePhase = "Pending"
	OpeaCachePhasePVCCreated OpeaCachePhase = "PVCCreated"
	OpeaCachePhaseExecuting  OpeaCachePhase = "Executing"
	OpeaCachePhaseReady      OpeaCachePhase = "Ready"
	OpeaCachePhaseFailed     OpeaCachePhase = "Failed"
)

// OpeaCacheStatus defines the observed state of OpeaCache.
type OpeaCacheStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	CacheStates `json:"cacheStates,omitempty"`
	// +optional
	SourceStates `json:"sourceStates,omitempty"`
	// +optional
	Phase OpeaCachePhase `json:"phase,omitempty"`
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

type CacheStates struct {
	CacheCapacity    resource.Quantity `json:"cacheCapacity,omitempty"`
	Cached           resource.Quantity `json:"cached,omitempty"`
	CachedPercentage string            `json:"cachedPercentage,omitempty"`
}

type SourceStates []SourceState

type SourceState struct {
	Name string `json:"name,omitempty"`
	Size string `json:"size,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="CachePercentage",type=string,JSONPath=`.status.cacheStates.cachedPercentage`
// OpeaCache is the Schema for the opeacaches API.
type OpeaCache struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpeaCacheSpec   `json:"spec,omitempty"`
	Status OpeaCacheStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpeaCacheList contains a list of OpeaCache.
type OpeaCacheList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpeaCache `json:"items"`
}

func (c *OpeaCache) GetPVCName() string {
	return fmt.Sprintf("%s-%s", c.Name, "pvc")
}

func init() {
	SchemeBuilder.Register(&OpeaCache{}, &OpeaCacheList{})
}
