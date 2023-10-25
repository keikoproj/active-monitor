package controllers

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	activemonitorv1alpha1 "github.com/keikoproj/active-monitor/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
)

var (
	healthCheckNamespace = "health"
	healthCheckName      = "inline-monitor-remedy"
	healthCheckKey       = types.NamespacedName{Name: healthCheckName, Namespace: healthCheckNamespace}
	healthCheckNameNs    = "inline-monitor-remedy-namespace"
	healthCheckKeyNs     = types.NamespacedName{Name: healthCheckNameNs, Namespace: healthCheckNamespace}
	healthCheckNamePause = "inline-hello-pause"
	healthCheckKeyPause  = types.NamespacedName{Name: healthCheckNamePause, Namespace: healthCheckNamespace}
)

const timeout = time.Second * 60

var _ = Describe("Active-Monitor Controller", func() {

	Describe("healthCheck CR can be reconciled at cluster level", func() {
		var instance *activemonitorv1alpha1.HealthCheck
		It("instance should be parsable", func() {
			//healthCheckYaml, err := ioutil.ReadFile("../examples/inlineHello.yaml")
			healthCheckYaml, err := ioutil.ReadFile("../../examples/bdd/inlineMemoryRemedyUnitTest.yaml")
			Expect(err).ToNot(HaveOccurred())
			instance, err = parseHealthCheckYaml(healthCheckYaml)
			Expect(err).ToNot(HaveOccurred())
			Expect(instance).To(BeAssignableToTypeOf(&activemonitorv1alpha1.HealthCheck{}))
			Expect(instance.GetName()).To(Equal(healthCheckName))
		})

		It("instance should be reconciled", func() {
			instance.SetNamespace(healthCheckNamespace)
			err := k8sClient.Create(context.TODO(), instance)
			if apierrors.IsInvalid(err) {
				log.Error(err, "failed to create object, got an invalid object error")
				return
			}
			Expect(err).NotTo(HaveOccurred())
			defer k8sClient.Delete(context.TODO(), instance)

			Eventually(func() error {
				if err := k8sClient.Get(context.TODO(), healthCheckKey, instance); err != nil {
					return err
				}

				if instance.Status.StartedAt != nil && instance.Status.SuccessCount+instance.Status.FailedCount >= 3 {
					return nil
				}
				return fmt.Errorf("HealthCheck is not valid")
			}, timeout).Should(Succeed())

			By("Verify healthCheck has been reconciled by checking for status")
			Expect(instance.Status.ErrorMessage).ShouldNot(BeEmpty())
		})
	})

	Describe("healthCheck CR can be reconciled at namespace level", func() {
		var instance *activemonitorv1alpha1.HealthCheck

		It("instance should be parsable", func() {
			//healthCheckYaml, err := ioutil.ReadFile("../examples/inlineHello.yaml")
			healthCheckYaml, err := ioutil.ReadFile("../../examples/bdd/inlineMemoryRemedyUnitTest_Namespace.yaml")
			Expect(err).ToNot(HaveOccurred())

			instance, err = parseHealthCheckYaml(healthCheckYaml)
			Expect(err).ToNot(HaveOccurred())
			Expect(instance).To(BeAssignableToTypeOf(&activemonitorv1alpha1.HealthCheck{}))
			Expect(instance.GetName()).To(Equal(healthCheckNameNs))
		})

		It("instance should be reconciled", func() {
			instance.SetNamespace(healthCheckNamespace)
			err := k8sClient.Create(context.TODO(), instance)
			if apierrors.IsInvalid(err) {
				log.Error(err, "failed to create object, got an invalid object error")
				return
			}
			Expect(err).NotTo(HaveOccurred())
			defer k8sClient.Delete(context.TODO(), instance)

			Eventually(func() error {
				if err := k8sClient.Get(context.TODO(), healthCheckKeyNs, instance); err != nil {
					return err
				}

				if instance.Status.StartedAt != nil {
					return nil
				}
				return fmt.Errorf("HealthCheck is not valid")
			}, timeout).Should(Succeed())

			By("Verify healthCheck has been reconciled by checking for status")
			Expect(instance.Status.ErrorMessage).ShouldNot(BeEmpty())
		})
	})

	Describe("healthCheck CR will be paused with repeatAfterSec set to 0", func() {
		var instance *activemonitorv1alpha1.HealthCheck

		It("instance should be parsable", func() {
			healthCheckYaml, err := ioutil.ReadFile("../../examples/bdd/inlineHelloTest.yaml")
			Expect(err).ToNot(HaveOccurred())

			instance, err = parseHealthCheckYaml(healthCheckYaml)
			Expect(err).ToNot(HaveOccurred())
			Expect(instance).To(BeAssignableToTypeOf(&activemonitorv1alpha1.HealthCheck{}))
			Expect(instance.GetName()).To(Equal(healthCheckNamePause))
		})

		It("instance should be reconciled", func() {
			instance.SetNamespace(healthCheckNamespace)
			err := k8sClient.Create(context.TODO(), instance)
			if apierrors.IsInvalid(err) {
				log.Error(err, "failed to create object, got an invalid object error")
				return
			}
			Expect(err).NotTo(HaveOccurred())
			defer k8sClient.Delete(context.TODO(), instance)

			Eventually(func() error {
				if err := k8sClient.Get(context.TODO(), healthCheckKeyPause, instance); err != nil {
					return err
				}

				if instance.Status.Status == "Stopped" {
					return nil
				}
				return fmt.Errorf("HealthCheck is not valid")
			}, timeout).Should(Succeed())

			By("Verify healthCheck has been reconciled by checking for status")
			Expect(instance.Status.ErrorMessage).ShouldNot(BeEmpty())
		})
	})
})

func parseHealthCheckYaml(data []byte) (*activemonitorv1alpha1.HealthCheck, error) {
	var err error
	o := &unstructured.Unstructured{}
	err = yaml.Unmarshal(data, &o.Object)
	if err != nil {
		return nil, err
	}
	a := &activemonitorv1alpha1.HealthCheck{}
	err = scheme.Scheme.Convert(o, a, 0)
	if err != nil {
		return nil, err
	}

	return a, nil
}

func TestHealthCheckReconciler_IsStorageError(t *testing.T) {
	c := &HealthCheckReconciler{}

	type test struct {
		name string
		err  error
		want bool
	}
	table := []test{
		{
			name: "got storage error",
			err:  errors.New("Operation cannot be fulfilled on pods: StorageError: invalid object, Code: 4, UID in object meta: "),
			want: true,
		},
		{
			name: "no storage error",
			err:  errors.New("Reconciler error"),
			want: false,
		},
	}
	for _, tt := range table {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := c.IsStorageError(tt.err)
			require.Equal(t, got, tt.want)
		})
	}
}
