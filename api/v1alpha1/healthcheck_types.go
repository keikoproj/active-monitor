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
	"reflect"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Important: Run "make" to regenerate code after modifying this file

// HealthCheckSpec defines the desired state of HealthCheck
// Either RepeatAfterSec or Schedule must be defined for the health check to run
type HealthCheckSpec struct {
	RepeatAfterSec int            `json:"repeatAfterSec,omitempty"`
	Description    string         `json:"description,omitempty"`
	Workflow       Workflow       `json:"workflow"`
	Level          string         `json:"level,omitempty"`    // defines if a workflow runs in a Namespace or Cluster level
	Schedule       ScheduleSpec   `json:"schedule,omitempty"` // Schedule defines schedule rules to run HealthCheck
	RemedyWorkflow RemedyWorkflow `json:"remedyworkflow,omitempty"`
}

// HealthCheckStatus defines the observed state of HealthCheck
type HealthCheckStatus struct {
	ErrorMessage           string       `json:"errorMessage,omitempty"`
	RemedyErrorMessage     string       `json:"remedyErrorMessage,omitempty"`
	StartedAt              *metav1.Time `json:"startedAt,omitempty"`
	FinishedAt             *metav1.Time `json:"finishedAt,omitempty"`
	LastFailedAt           *metav1.Time `json:"lastFailedAt,omitempty"`
	RemedyStartedAt        *metav1.Time `json:"remedyTriggeredAt,omitempty"`
	RemedyFinishedAt       *metav1.Time `json:"remedyFinishedAt,omitempty"`
	RemedyLastFailedAt     *metav1.Time `json:"remedyLastFailedAt,omitempty"`
	LastFailedWorkflow     string       `json:"lastFailedWorkflow,omitempty"`
	LastSuccessfulWorkflow string       `json:"lastSuccessfulWorkflow,omitempty"`
	SuccessCount           int          `json:"successCount"`
	FailedCount            int          `json:"failedCount"`
	RemedySuccessCount     int          `json:"remedySuccessCount,omitempty"`
	RemedyFailedCount      int          `json:"remedyFailedCount,omitempty"`
	RemedyTotalRuns        int          `json:"remedyTotalRuns,omitempty"`
	TotalHealthCheckRuns   int          `json:"totalHealthCheckRuns,omitempty"`
	Status                 string       `json:"status,omitempty"`
	RemedyStatus           string       `json:"remedyStatus,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=healthchecks,scope=Namespaced,shortName=hc;hcs
// +kubebuilder:printcolumn:name="LATEST STATUS",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="SUCCESS CNT  ",type=string,JSONPath=`.status.successCount`
// +kubebuilder:printcolumn:name="FAIL CNT",type=string,JSONPath=`.status.failedCount`
// +kubebuilder:printcolumn:name="REMEDY SUCCESS CNT  ",type=string,JSONPath=`.status.remedySuccessCount`
// +kubebuilder:printcolumn:name="REMEDY FAIL CNT",type=string,JSONPath=`.status.remedyFailedCount`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

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

// Workflow struct describes a Remedy workflow
type RemedyWorkflow struct {
	GenerateName string          `json:"generateName,omitempty"`
	Resource     *ResourceObject `json:"resource,omitempty"`
}

func (w RemedyWorkflow) IsEmpty() bool {
	return reflect.DeepEqual(w, RemedyWorkflow{})
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

// ScheduleSpec contains the cron expression
type ScheduleSpec struct {
	// cron expressions can be found here: https://godoc.org/github.com/robfig/cron
	Cron string `json:"cron,omitempty"`
}

func init() {
	SchemeBuilder.Register(&HealthCheck{}, &HealthCheckList{})
}
