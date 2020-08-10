package controllers

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	activemonitorv1alpha1 "github.com/keikoproj/active-monitor/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
)

var (
	healthCheckNamespace = "health"
	healthCheckName      = "inline-hello"
	healthCheckKey       = types.NamespacedName{Name: healthCheckName, Namespace: healthCheckNamespace}
)

const timeout = time.Second * 30

var _ = Describe("Active-Monitor Controller", func() {

	Describe("healthCheck CR can be reconciled", func() {
		var instance *activemonitorv1alpha1.HealthCheck

		It("instance should be parsable", func() {
			healthCheckYaml, err := ioutil.ReadFile("/Users/rhari/go/src/github.com/RaviHari/active-monitor/examples/inlineHello.yaml")
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

				if instance.ObjectMeta.DeletionTimestamp.IsZero() {
					return nil
				}
				return fmt.Errorf("HealthCheck is not valid")
			}, timeout).Should(Succeed())

			By("Verify healthCheck has been reconciled by checking for checksum status")
			Expect(instance.Status.ErrorMessage).Should(BeEmpty())
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
