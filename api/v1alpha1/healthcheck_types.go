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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Important: Run "make" to regenerate code after modifying this file

// HealthCheckSpec defines the desired state of HealthCheck
type HealthCheckSpec struct {
	RepeatAfterSec int      `json:"repeatAfterSec"`
	Description    string   `json:"description,omitempty"`
	Workflow       Workflow `json:"workflow"`
	Level          string   `json:"level,omitempty"` // defines if a workflow runs in at Namespace or Cluster level
}

// HealthCheckStatus defines the observed state of HealthCheck
type HealthCheckStatus struct {
	ErrorMessage           string       `json:"errorMessage,omitempty"`
	FinishedAt             *metav1.Time `json:"finishedAt,omitempty"`
	LastFailedAt           *metav1.Time `json:"lastFailedAt,omitempty"`
	LastFailedWorkflow     string       `json:"lastFailedWorkflow,omitempty"`
	LastSuccessfulWorkflow string       `json:"lastSuccessfulWorkflow,omitempty"`
	SuccessCount           int          `json:"successCount"`
	FailedCount            int          `json:"failedCount"`
	Status                 string       `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=healthchecks,scope=Namespaced,shortName=hc;hcs

// HealthCheck is the Schema for the healthchecks API
type HealthCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HealthCheckSpec   `json:"spec,omitempty"`
	Status HealthCheckStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HealthCheckList contains a list of HealthCheck
type HealthCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HealthCheck `json:"items"`
}

// Workflow struct describes an Argo workflow
type Workflow struct {
	GenerateName string          `json:"generateName,omitempty"`
	Resource     *ResourceObject `json:"resource,omitempty"`
}

// ResourceObject is the resource object to create on kubernetes
type ResourceObject struct {
	// Namespace in which to create this object
	// defaults to the service account namespace
	Namespace      string `json:"namespace"`
	ServiceAccount string `json:"serviceAccount,omitempty"`
	// Source of the K8 resource file(s)
	Source ArtifactLocation `json:"source"`
}

// ArtifactLocation describes the source location for an external artifact
type ArtifactLocation struct {
	Inline *string       `json:"inline,omitempty"`
	File   *FileArtifact `json:"file,omitempty"`
	URL    *URLArtifact  `json:"url,omitempty"`
}

// FileArtifact contains information about an artifact in a filesystem
type FileArtifact struct {
	Path string `json:"path,omitempty"`
}

// URLArtifact contains information about an artifact at an http endpoint.
type URLArtifact struct {
	Path       string `json:"path,omitempty"`
	VerifyCert bool   `json:"verifyCert,omitempty"`
}

func init() {
	SchemeBuilder.Register(&HealthCheck{}, &HealthCheckList{})
}
