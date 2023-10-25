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
	"path/filepath"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	activemonitorv1alpha1 "github.com/keikoproj/active-monitor/api/v1alpha1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

var mgr manager.Manager
var ctx = context.Background()
var wg *sync.WaitGroup
var log logr.Logger

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	done := make(chan interface{})
	go func() {
		log = zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter))
		logf.SetLogger(log)

		By("bootstrapping test environment")
		testEnv = &envtest.Environment{
			CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
		}

		cfg, err := testEnv.Start()
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg).ToNot(BeNil())

		err = activemonitorv1alpha1.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		// +kubebuilder:scaffold:scheme

		By("starting reconciler and manager")
		k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
		Expect(err).ToNot(HaveOccurred())
		Expect(k8sClient).ToNot(BeNil())
		err = k8sClient.Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "health"}})
		Expect(err).To(BeNil())

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme:             scheme.Scheme,
			MetricsBindAddress: ":8080",
			LeaderElection:     false,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(mgr).ToNot(BeNil())

		err = NewHealthCheckReconciler(mgr, ctrl.Log.WithName("controllers").WithName("HealthCheck"), 10).SetupWithManager(mgr)
		Expect(err).ToNot(HaveOccurred())

		wg = StartTestManager(mgr)

		close(done) //signifies the code is done
	}()
	Eventually(done, 60).Should(BeClosed())
})

var _ = AfterSuite(func() {
	By("stopping manager")
	ctx.Done()

	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func StartTestManager(mgr manager.Manager) *sync.WaitGroup {
	wg := &sync.WaitGroup{}
	go func() {
		wg.Add(1)
		//mgr.Start(stop)
		Expect(mgr.Start(ctx)).ToNot(HaveOccurred())
		wg.Done()
	}()
	return wg
}
