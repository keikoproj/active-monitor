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

package controllers

import (
	"context"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	activemonitorv1alpha1 "github.com/keikoproj/active-monitor/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
)

// newTestReconciler returns a minimal HealthCheckReconciler for pure unit tests.
// The kubeclient field is left nil — RBAC method tests pass a fake clientset
// directly as the kubernetes.Interface argument.
func newTestReconciler() *HealthCheckReconciler {
	return &HealthCheckReconciler{
		Recorder:  record.NewFakeRecorder(100),
		Log:       logr.Discard(),
		TimerLock: sync.RWMutex{},
	}
}

// newHC builds a minimal HealthCheck for use in unit tests.
func newHC(name, namespace string) *activemonitorv1alpha1.HealthCheck {
	return &activemonitorv1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
}

// --- ContainsEqualFoldSubstring ---

func TestContainsEqualFoldSubstring_Match(t *testing.T) {
	r := newTestReconciler()
	assert.True(t, r.ContainsEqualFoldSubstring("StorageError: invalid object", "storageerror"))
}

func TestContainsEqualFoldSubstring_CaseInsensitive(t *testing.T) {
	r := newTestReconciler()
	assert.True(t, r.ContainsEqualFoldSubstring("HELLO WORLD", "hello"))
	assert.True(t, r.ContainsEqualFoldSubstring("hello world", "HELLO"))
}

func TestContainsEqualFoldSubstring_NoMatch(t *testing.T) {
	r := newTestReconciler()
	assert.False(t, r.ContainsEqualFoldSubstring("normal error", "storageerror"))
}

func TestContainsEqualFoldSubstring_EmptySubstr(t *testing.T) {
	r := newTestReconciler()
	// empty substring is always contained
	assert.True(t, r.ContainsEqualFoldSubstring("anything", ""))
}

func TestContainsEqualFoldSubstring_BothEmpty(t *testing.T) {
	r := newTestReconciler()
	assert.True(t, r.ContainsEqualFoldSubstring("", ""))
}

// --- IsStorageError ---

func TestIsStorageError_True(t *testing.T) {
	r := newTestReconciler()
	assert.True(t, r.IsStorageError(&simpleError{"StorageError: invalid object, Code: 2"}))
}

func TestIsStorageError_False(t *testing.T) {
	r := newTestReconciler()
	assert.False(t, r.IsStorageError(&simpleError{"some other error"}))
}

type simpleError struct{ msg string }

func (e *simpleError) Error() string { return e.msg }

// --- parseWorkflowFromHealthcheck error paths ---

func TestParseWorkflowFromHealthcheck_InvalidYAML_ReturnsError(t *testing.T) {
	r := newTestReconciler()
	hc := newHC("parse-invalid-yaml", "default")
	badYAML := "this: is: not: valid: yaml: {"
	hc.Spec.Workflow.Resource = &activemonitorv1alpha1.ResourceObject{
		Source: activemonitorv1alpha1.ArtifactLocation{Inline: &badYAML},
	}
	uwf := &unstructured.Unstructured{}
	uwf.SetUnstructuredContent(map[string]interface{}{"spec": map[string]interface{}{}})

	err := r.parseWorkflowFromHealthcheck(logr.Discard(), hc, uwf)
	assert.Error(t, err)
}

func TestParseWorkflowFromHealthcheck_UnknownArtifact_ReturnsError(t *testing.T) {
	// File artifact is unimplemented — GetArtifactReader returns "unknown artifact location"
	r := newTestReconciler()
	hc := newHC("parse-unknown-artifact", "default")
	hc.Spec.Workflow.Resource = &activemonitorv1alpha1.ResourceObject{
		Source: activemonitorv1alpha1.ArtifactLocation{
			File: &activemonitorv1alpha1.FileArtifact{Path: "/some/path"},
		},
	}
	uwf := &unstructured.Unstructured{}

	err := r.parseWorkflowFromHealthcheck(logr.Discard(), hc, uwf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown artifact location")
}

func TestParseWorkflowFromHealthcheck_MissingSpec_Panics(t *testing.T) {
	// Valid YAML but with no "spec" key. parseWorkflowFromHealthcheck panics at
	// line 892 because data["spec"] is nil and is immediately type-asserted to
	// map[string]interface{} without a nil guard. This test documents the panic
	// so that when a nil guard is added the test can be changed to assert NoError.
	r := newTestReconciler()
	hc := newHC("parse-missing-spec", "default")
	noSpec := "apiVersion: argoproj.io/v1alpha1\nkind: Workflow\nmetadata:\n  generateName: test-\n"
	hc.Spec.Workflow.Resource = &activemonitorv1alpha1.ResourceObject{
		Source: activemonitorv1alpha1.ArtifactLocation{Inline: &noSpec},
	}
	hc.Spec.RepeatAfterSec = 30
	uwf := &unstructured.Unstructured{}
	uwf.SetUnstructuredContent(map[string]interface{}{})

	assert.Panics(t, func() {
		_ = r.parseWorkflowFromHealthcheck(logr.Discard(), hc, uwf)
	})
}

// --- RBAC verb scope: health vs. remedy ClusterRole ---

func TestCreateClusterRole_HealthCheck_ReadOnlyVerbs(t *testing.T) {
	r := newTestReconciler()
	cs := fake.NewSimpleClientset()
	hc := newHC("rbac-health", "default")

	name, err := r.createClusterRole(context.Background(), cs, logr.Discard(), hc, "test-health-cluster-role")
	require.NoError(t, err)
	assert.Equal(t, "test-health-cluster-role", name)

	cr, err := cs.RbacV1().ClusterRoles().Get(context.Background(), "test-health-cluster-role", metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, cr.Rules, 1)
	verbs := cr.Rules[0].Verbs
	assert.Contains(t, verbs, "get")
	assert.Contains(t, verbs, "list")
	assert.Contains(t, verbs, "watch")
	assert.NotContains(t, verbs, "create")
	assert.NotContains(t, verbs, "update")
	assert.NotContains(t, verbs, "patch")
	assert.NotContains(t, verbs, "delete")
}

func TestCreateRemedyClusterRole_HasWriteVerbs(t *testing.T) {
	r := newTestReconciler()
	cs := fake.NewSimpleClientset()
	hc := newHC("rbac-remedy", "default")

	name, err := r.createRemedyClusterRole(context.Background(), cs, logr.Discard(), hc, "test-remedy-cluster-role")
	require.NoError(t, err)
	assert.Equal(t, "test-remedy-cluster-role", name)

	cr, err := cs.RbacV1().ClusterRoles().Get(context.Background(), "test-remedy-cluster-role", metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, cr.Rules, 1)
	verbs := cr.Rules[0].Verbs
	assert.Contains(t, verbs, "get")
	assert.Contains(t, verbs, "list")
	assert.Contains(t, verbs, "watch")
	assert.Contains(t, verbs, "create")
	assert.Contains(t, verbs, "update")
	assert.Contains(t, verbs, "patch")
	assert.Contains(t, verbs, "delete")
}

func TestCreateClusterRole_Idempotent(t *testing.T) {
	r := newTestReconciler()
	cs := fake.NewSimpleClientset()
	hc := newHC("rbac-idem", "default")

	name1, err := r.createClusterRole(context.Background(), cs, logr.Discard(), hc, "idem-role")
	require.NoError(t, err)
	name2, err := r.createClusterRole(context.Background(), cs, logr.Discard(), hc, "idem-role")
	require.NoError(t, err)
	assert.Equal(t, name1, name2)
}

// --- DeleteClusterRole: WfManagedByLabelKey guard ---

func TestDeleteClusterRole_ManagedRole_IsDeleted(t *testing.T) {
	r := newTestReconciler()
	cs := fake.NewSimpleClientset()
	hc := newHC("del-managed", "default")

	// Create via controller so it gets the managed-by label
	_, err := r.createClusterRole(context.Background(), cs, logr.Discard(), hc, "managed-role")
	require.NoError(t, err)

	err = r.DeleteClusterRole(context.Background(), cs, logr.Discard(), hc, "managed-role")
	require.NoError(t, err)

	_, err = cs.RbacV1().ClusterRoles().Get(context.Background(), "managed-role", metav1.GetOptions{})
	assert.Error(t, err, "managed ClusterRole should have been deleted")
}

func TestDeleteClusterRole_UnmanagedRole_IsNotDeleted(t *testing.T) {
	r := newTestReconciler()
	cs := fake.NewSimpleClientset()
	hc := newHC("del-unmanaged", "default")

	// Create a ClusterRole WITHOUT the managed-by label (simulating pre-existing infra)
	require.NoError(t, createUnmanagedClusterRole(cs, "unmanaged-role"))

	err := r.DeleteClusterRole(context.Background(), cs, logr.Discard(), hc, "unmanaged-role")
	require.NoError(t, err)

	_, err = cs.RbacV1().ClusterRoles().Get(context.Background(), "unmanaged-role", metav1.GetOptions{})
	assert.NoError(t, err, "unmanaged ClusterRole should NOT have been deleted")
}

// createUnmanagedClusterRole creates a ClusterRole without the managed-by label,
// simulating a pre-existing resource that active-monitor should not touch.
func createUnmanagedClusterRole(cs kubernetes.Interface, name string) error {
	_, err := cs.RbacV1().ClusterRoles().Create(context.Background(), &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"get"}},
		},
	}, metav1.CreateOptions{})
	return err
}

// --- ServiceAccount collision: same SA name for health and remedy ---

func TestCreateRBACForWorkflow_SACollision_RemedyGetsSuffix(t *testing.T) {
	// When hcSa == remedySa, createRBACForWorkflow should rename the remedy SA
	// by appending "-remedy" so it receives separate (write) permissions.
	// The rename (line 271) happens before the first kubeclient call (line 286),
	// so we can observe the mutation even though the function panics afterward
	// (r.kubeclient is nil in newTestReconciler). We recover the panic and check
	// the value was already mutated.
	hc := &activemonitorv1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "sa-collision", Namespace: "health"},
		Spec: activemonitorv1alpha1.HealthCheckSpec{
			RepeatAfterSec: 30,
			Level:          "cluster",
			Workflow: activemonitorv1alpha1.Workflow{
				GenerateName: "collision-",
				Resource: &activemonitorv1alpha1.ResourceObject{
					Namespace:      "health",
					ServiceAccount: "shared-sa",
					Source:         activemonitorv1alpha1.ArtifactLocation{Inline: strPtr(inlineWorkflowSpec)},
				},
			},
			RemedyWorkflow: activemonitorv1alpha1.RemedyWorkflow{
				GenerateName: "collision-remedy-",
				Resource: &activemonitorv1alpha1.ResourceObject{
					Namespace:      "health",
					ServiceAccount: "shared-sa", // intentionally same as health SA
					Source:         activemonitorv1alpha1.ArtifactLocation{Inline: strPtr(inlineWorkflowSpec)},
				},
			},
		},
	}

	require.Equal(t, "shared-sa", hc.Spec.RemedyWorkflow.Resource.ServiceAccount, "precondition: SAs are the same")

	r := newTestReconciler()
	func() {
		defer func() { recover() }() // tolerate panic from nil kubeclient after the rename
		_ = r.createRBACForWorkflow(context.Background(), logr.Discard(), hc, healthcheck)
	}()

	assert.Equal(t, "shared-sa-remedy", hc.Spec.RemedyWorkflow.Resource.ServiceAccount,
		"remedy SA should be renamed to avoid collision with health SA")
}
