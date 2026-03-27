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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsfilters "sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	activemonitorv1alpha1 "github.com/keikoproj/active-monitor/api/v1alpha1"
	"github.com/keikoproj/active-monitor/internal/controllers"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(activemonitorv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

// managerOptions holds the configuration parsed from command-line flags.
type managerOptions struct {
	metricsAddr          string
	probeAddr            string
	enableLeaderElection bool
	maxParallel          int
	secureMetrics        bool
}

// run contains the core application logic, extracted from main() for testability.
// It accepts a REST config and a context so that tests can supply their own.
// If restConfig is nil, it falls back to ctrl.GetConfigOrDie().
func run(ctx context.Context, restConfig *rest.Config, opts managerOptions) error {
	if restConfig == nil {
		restConfig = ctrl.GetConfigOrDie()
	}

	// Configure metrics options
	metricsServerOptions := server.Options{
		BindAddress: opts.metricsAddr,
	}

	// Enable authentication and authorization for metrics when secure metrics is enabled
	if opts.secureMetrics {
		metricsServerOptions.FilterProvider = metricsfilters.WithAuthenticationAndAuthorization
	}

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		HealthProbeBindAddress: opts.probeAddr,
		LeaderElection:         opts.enableLeaderElection,
		LeaderElectionID:       "689451f8.keikoproj.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		return fmt.Errorf("unable to start manager: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("unable to get dynamic client: %w", err)
	}

	if err = (&controllers.HealthCheckReconciler{
		Client:      mgr.GetClient(),
		DynClient:   dynClient,
		Recorder:    mgr.GetEventRecorderFor("HealthCheck"),
		Log:         ctrl.Log.WithName("controllers").WithName("HealthCheck"),
		MaxParallel: opts.maxParallel,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller HealthCheck: %w", err)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("problem running manager: %w", err)
	}
	return nil
}

func main() {
	var opts managerOptions

	flag.StringVar(&opts.metricsAddr, "metrics-bind-address", ":8443", "The address the metric endpoint binds to.")
	flag.BoolVar(&opts.secureMetrics, "metrics-secure", true, "Enable authentication and authorization for the metrics endpoint.")
	flag.BoolVar(&opts.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&opts.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.IntVar(&opts.maxParallel, "max-workers", 10, "The number of maximum parallel reconciles")

	zapOpts := zap.Options{
		Development: true,
	}
	zapOpts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	if err := run(ctrl.SetupSignalHandler(), nil, opts); err != nil {
		setupLog.Error(err, "application failed")
		os.Exit(1)
	}
}
