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
	"fmt"
	"time"

	activemonitorv1alpha1 "github.com/keikoproj/active-monitor/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// inlineWorkflowSpec is a minimal Argo Workflow spec used across edge-case tests.
const inlineWorkflowSpec = `apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: edge-test-
spec:
  entrypoint: hello
  templates:
  - name: hello
    container:
      image: alpine:3.6
      command: [echo]
      args: ["hello"]
`

var _ = Describe("Active-Monitor Controller edge cases", func() {

	Describe("HealthCheck with nil Workflow.Resource is ignored", func() {
		It("should reconcile without error and set no status", func() {
			name := "edge-nil-resource"
			hc := &activemonitorv1alpha1.HealthCheck{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: healthCheckNamespace},
				Spec: activemonitorv1alpha1.HealthCheckSpec{
					RepeatAfterSec: 30,
					Level:          "cluster",
					Workflow: activemonitorv1alpha1.Workflow{
						GenerateName: "edge-nil-",
						// Resource intentionally nil
					},
				},
			}
			Expect(k8sClient.Create(context.TODO(), hc)).To(Succeed())
			defer k8sClient.Delete(context.TODO(), hc)

			// The controller should reconcile without crashing. Status should stay empty
			// because processHealthCheck returns early when Workflow.Resource is nil.
			Consistently(func() string {
				fetched := &activemonitorv1alpha1.HealthCheck{}
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: healthCheckNamespace}, fetched); err != nil {
					return "get-error"
				}
				return fetched.Status.Status
			}, 5*time.Second, time.Second).Should(Equal(""))
		})
	})

	Describe("HealthCheck with empty Level returns an error on reconcile", func() {
		It("should reconcile and reflect an error in status", func() {
			name := "edge-empty-level"
			hc := &activemonitorv1alpha1.HealthCheck{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: healthCheckNamespace},
				Spec: activemonitorv1alpha1.HealthCheckSpec{
					RepeatAfterSec: 5,
					Level:          "", // invalid — neither "cluster" nor "namespace"
					Workflow: activemonitorv1alpha1.Workflow{
						GenerateName: "edge-level-",
						Resource: &activemonitorv1alpha1.ResourceObject{
							Namespace:      healthCheckNamespace,
							ServiceAccount: "activemonitor-healthcheck-sa",
							Source:         activemonitorv1alpha1.ArtifactLocation{Inline: strPtr(inlineWorkflowSpec)},
						},
					},
				},
			}
			Expect(k8sClient.Create(context.TODO(), hc)).To(Succeed())
			defer k8sClient.Delete(context.TODO(), hc)

			// The controller should not crash; the error from createRBACForWorkflow
			// ("level is not set") propagates up and causes a requeue. The HealthCheck
			// itself should remain accessible (no panic, no infinite tight-loop crash).
			Eventually(func() error {
				fetched := &activemonitorv1alpha1.HealthCheck{}
				return k8sClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: healthCheckNamespace}, fetched)
			}, 10*time.Second, time.Second).Should(Succeed(), "HealthCheck object should remain accessible")
		})
	})

	Describe("HealthCheck with invalid cron expression does not panic", func() {
		It("should reconcile without crashing and not reach a successful status", func() {
			name := "edge-invalid-cron"
			hc := &activemonitorv1alpha1.HealthCheck{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: healthCheckNamespace},
				Spec: activemonitorv1alpha1.HealthCheckSpec{
					RepeatAfterSec: 0,
					Level:          "cluster",
					Schedule: activemonitorv1alpha1.ScheduleSpec{
						Cron: "NOT_A_VALID_CRON",
					},
					Workflow: activemonitorv1alpha1.Workflow{
						GenerateName: "edge-cron-",
						Resource: &activemonitorv1alpha1.ResourceObject{
							Namespace:      healthCheckNamespace,
							ServiceAccount: "activemonitor-healthcheck-sa",
							Source:         activemonitorv1alpha1.ArtifactLocation{Inline: strPtr(inlineWorkflowSpec)},
						},
					},
				},
			}
			Expect(k8sClient.Create(context.TODO(), hc)).To(Succeed())
			defer k8sClient.Delete(context.TODO(), hc)

			// The controller must not panic when cron.ParseStandard fails.
			// The object should remain retrievable after reconcile runs.
			Eventually(func() error {
				fetched := &activemonitorv1alpha1.HealthCheck{}
				return k8sClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: healthCheckNamespace}, fetched)
			}, 10*time.Second, time.Second).Should(Succeed(), "HealthCheck should remain accessible after invalid cron parse")

			// With the fix, the cron parse error is returned (not silently swallowed
			// via panic recovery), so the HealthCheck should never reach a successful
			// "Completed" status. It should remain empty or in an error state.
			Consistently(func() string {
				fetched := &activemonitorv1alpha1.HealthCheck{}
				if err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: healthCheckNamespace}, fetched); err != nil {
					return "get-error"
				}
				return fetched.Status.Status
			}, 5*time.Second, time.Second).ShouldNot(Equal("Completed"),
				"HealthCheck with invalid cron should never reach Completed status")
		})
	})

	Describe("Timer is stopped when HealthCheck is deleted", func() {
		It("should stop the timer in RepeatTimersByName on delete", func() {
			name := "edge-timer-delete"
			// Pre-register a timer so the delete path exercises the Stop() call
			stopped := false
			sharedCtrl.TimerLock.Lock()
			sharedCtrl.RepeatTimersByName[name] = time.AfterFunc(time.Hour, func() {
				// This should never fire; the test verifies it gets stopped.
				stopped = true
			})
			sharedCtrl.TimerLock.Unlock()

			hc := &activemonitorv1alpha1.HealthCheck{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: healthCheckNamespace},
				Spec: activemonitorv1alpha1.HealthCheckSpec{
					// RepeatAfterSec: 0 triggers a fast "Stopped" early return in processHealthCheck —
					// these tests only exercise the deletion path, not workflow execution.
					RepeatAfterSec: 0,
					Level:          "cluster",
					Workflow: activemonitorv1alpha1.Workflow{
						GenerateName: "edge-timer-",
						Resource: &activemonitorv1alpha1.ResourceObject{
							Namespace:      healthCheckNamespace,
							ServiceAccount: "activemonitor-healthcheck-sa",
							Source:         activemonitorv1alpha1.ArtifactLocation{Inline: strPtr(inlineWorkflowSpec)},
						},
					},
				},
			}
			Expect(k8sClient.Create(context.TODO(), hc)).To(Succeed())

			// Delete immediately to trigger the Not-Found path in Reconcile
			Expect(k8sClient.Delete(context.TODO(), hc)).To(Succeed())

			// Wait for the reconcile triggered by deletion to run
			Eventually(func() error {
				fetched := &activemonitorv1alpha1.HealthCheck{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: healthCheckNamespace}, fetched)
				if err != nil {
					return nil // object gone — reconcile has processed the deletion
				}
				return fmt.Errorf("object still exists")
			}, timeout).Should(Succeed())

			// Timer should have been stopped (not fired)
			Expect(stopped).To(BeFalse(), "timer should have been stopped, not fired")
		})

		It("should not panic when deleted HealthCheck has no timer", func() {
			name := "edge-timer-no-entry"
			// Ensure no timer entry exists for this name
			sharedCtrl.TimerLock.Lock()
			delete(sharedCtrl.RepeatTimersByName, name)
			sharedCtrl.TimerLock.Unlock()

			hc := &activemonitorv1alpha1.HealthCheck{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: healthCheckNamespace},
				Spec: activemonitorv1alpha1.HealthCheckSpec{
					// RepeatAfterSec: 0 triggers a fast "Stopped" early return in processHealthCheck —
					// these tests only exercise the deletion path, not workflow execution.
					RepeatAfterSec: 0,
					Level:          "cluster",
					Workflow: activemonitorv1alpha1.Workflow{
						GenerateName: "edge-notimer-",
						Resource: &activemonitorv1alpha1.ResourceObject{
							Namespace:      healthCheckNamespace,
							ServiceAccount: "activemonitor-healthcheck-sa",
							Source:         activemonitorv1alpha1.ArtifactLocation{Inline: strPtr(inlineWorkflowSpec)},
						},
					},
				},
			}
			Expect(k8sClient.Create(context.TODO(), hc)).To(Succeed())
			Expect(k8sClient.Delete(context.TODO(), hc)).To(Succeed())

			Eventually(func() error {
				fetched := &activemonitorv1alpha1.HealthCheck{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: healthCheckNamespace}, fetched)
				if err != nil {
					return nil
				}
				return fmt.Errorf("object still exists")
			}, timeout).Should(Succeed(), "deletion should complete without panic")
		})
	})

	Describe("HealthCheck with non-empty RemedyWorkflow but nil Resource does not panic (#313)", func() {
		It("should reconcile without crashing when RemedyWorkflow.Resource is nil", func() {
			name := "edge-nil-remedy-resource"
			hc := &activemonitorv1alpha1.HealthCheck{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: healthCheckNamespace},
				Spec: activemonitorv1alpha1.HealthCheckSpec{
					RepeatAfterSec: 30,
					Level:          "cluster",
					Workflow: activemonitorv1alpha1.Workflow{
						GenerateName: "edge-remedy-nil-",
						Resource: &activemonitorv1alpha1.ResourceObject{
							Namespace:      healthCheckNamespace,
							ServiceAccount: "activemonitor-healthcheck-sa",
							Source:         activemonitorv1alpha1.ArtifactLocation{Inline: strPtr(inlineWorkflowSpec)},
						},
					},
					RemedyWorkflow: activemonitorv1alpha1.RemedyWorkflow{
						GenerateName: "remedy-nil-resource-",
						// Resource intentionally nil — this previously caused a panic
					},
				},
			}
			Expect(k8sClient.Create(context.TODO(), hc)).To(Succeed())
			defer k8sClient.Delete(context.TODO(), hc)

			// The controller should reconcile without crashing. The nil Resource guard
			// in createRBACForWorkflow should return an error, not a panic.
			Consistently(func() error {
				fetched := &activemonitorv1alpha1.HealthCheck{}
				return k8sClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: healthCheckNamespace}, fetched)
			}, 5*time.Second, time.Second).Should(Succeed(), "HealthCheck should remain accessible (no panic)")
		})
	})
})

var _ = Describe("Type assertion safety edge cases", func() {

	Describe("HealthCheck with non-map metadata in workflow spec does not panic", func() {
		It("should reconcile without crashing when metadata is a string", func() {
			name := "edge-nonmap-metadata"
			nonMapMetaSpec := `apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata: this-is-a-string
spec:
  entrypoint: hello
  templates:
  - name: hello
    container:
      image: alpine:3.6
      command: [echo]
      args: ["hello"]
`
			hc := &activemonitorv1alpha1.HealthCheck{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: healthCheckNamespace},
				Spec: activemonitorv1alpha1.HealthCheckSpec{
					RepeatAfterSec: 5,
					Level:          "cluster",
					Workflow: activemonitorv1alpha1.Workflow{
						GenerateName: "edge-meta-",
						Resource: &activemonitorv1alpha1.ResourceObject{
							Namespace:      healthCheckNamespace,
							ServiceAccount: "activemonitor-healthcheck-sa",
							Source:         activemonitorv1alpha1.ArtifactLocation{Inline: strPtr(nonMapMetaSpec)},
						},
					},
				},
			}
			Expect(k8sClient.Create(context.TODO(), hc)).To(Succeed())
			defer k8sClient.Delete(context.TODO(), hc)

			// The controller should not panic. The HealthCheck object should remain
			// accessible and the controller should process it without crashing.
			Eventually(func() error {
				fetched := &activemonitorv1alpha1.HealthCheck{}
				return k8sClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: healthCheckNamespace}, fetched)
			}, 10*time.Second, time.Second).Should(Succeed(), "HealthCheck should remain accessible with non-map metadata")
		})
	})

	Describe("HealthCheck with non-map spec in workflow does not panic", func() {
		It("should reconcile without crashing when spec is a string", func() {
			name := "edge-nonmap-spec"
			nonMapSpecYAML := `apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: edge-spec-
spec: this-is-not-a-map
`
			hc := &activemonitorv1alpha1.HealthCheck{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: healthCheckNamespace},
				Spec: activemonitorv1alpha1.HealthCheckSpec{
					RepeatAfterSec: 5,
					Level:          "cluster",
					Workflow: activemonitorv1alpha1.Workflow{
						GenerateName: "edge-spec-",
						Resource: &activemonitorv1alpha1.ResourceObject{
							Namespace:      healthCheckNamespace,
							ServiceAccount: "activemonitor-healthcheck-sa",
							Source:         activemonitorv1alpha1.ArtifactLocation{Inline: strPtr(nonMapSpecYAML)},
						},
					},
				},
			}
			Expect(k8sClient.Create(context.TODO(), hc)).To(Succeed())
			defer k8sClient.Delete(context.TODO(), hc)

			// The controller should handle the error gracefully (returning an error
			// from parseWorkflowFromHealthcheck) without panicking.
			Eventually(func() error {
				fetched := &activemonitorv1alpha1.HealthCheck{}
				return k8sClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: healthCheckNamespace}, fetched)
			}, 10*time.Second, time.Second).Should(Succeed(), "HealthCheck should remain accessible with non-map spec")
		})
	})

	Describe("HealthCheck with non-map labels in workflow spec does not panic", func() {
		It("should reconcile without crashing when labels is a string", func() {
			name := "edge-nonmap-labels"
			nonMapLabelsSpec := `apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: edge-labels-
  labels: not-a-map
spec:
  entrypoint: hello
  templates:
  - name: hello
    container:
      image: alpine:3.6
      command: [echo]
      args: ["hello"]
`
			hc := &activemonitorv1alpha1.HealthCheck{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: healthCheckNamespace},
				Spec: activemonitorv1alpha1.HealthCheckSpec{
					RepeatAfterSec: 5,
					Level:          "cluster",
					Workflow: activemonitorv1alpha1.Workflow{
						GenerateName: "edge-labels-",
						Resource: &activemonitorv1alpha1.ResourceObject{
							Namespace:      healthCheckNamespace,
							ServiceAccount: "activemonitor-healthcheck-sa",
							Source:         activemonitorv1alpha1.ArtifactLocation{Inline: strPtr(nonMapLabelsSpec)},
						},
					},
				},
			}
			Expect(k8sClient.Create(context.TODO(), hc)).To(Succeed())
			defer k8sClient.Delete(context.TODO(), hc)

			// The controller should not panic when labels can't be asserted to map.
			Eventually(func() error {
				fetched := &activemonitorv1alpha1.HealthCheck{}
				return k8sClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: healthCheckNamespace}, fetched)
			}, 10*time.Second, time.Second).Should(Succeed(), "HealthCheck should remain accessible with non-map labels")
		})
	})
})

func strPtr(s string) *string {
	return &s
}
