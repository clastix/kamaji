// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeconfiggenerator

import (
	"flag"
	"fmt"
	"io"
	"os"
	goRuntime "runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/clastix/kamaji/controllers"
	"github.com/clastix/kamaji/internal"
)

func NewCmd(scheme *runtime.Scheme) *cobra.Command {
	// CLI flags
	var (
		metricsBindAddress            string
		healthProbeBindAddress        string
		leaderElect                   bool
		controllerReconcileTimeout    time.Duration
		cacheResyncPeriod             time.Duration
		managerNamespace              string
		certificateExpirationDeadline time.Duration
	)

	cmd := &cobra.Command{
		Use:           "kubeconfig-generator",
		Short:         "Start the Kubeconfig Generator manager",
		SilenceErrors: false,
		SilenceUsage:  true,
		PreRunE: func(*cobra.Command, []string) error {
			// Avoid polluting stdout with useless details by the underlying klog implementations
			klog.SetOutput(io.Discard)
			klog.LogToStderr(false)

			if certificateExpirationDeadline < 24*time.Hour {
				return fmt.Errorf("certificate expiration deadline must be at least 24 hours")
			}

			return nil
		},
		RunE: func(*cobra.Command, []string) error {
			ctx := ctrl.SetupSignalHandler()

			setupLog := ctrl.Log.WithName("kubeconfig-generator")

			setupLog.Info(fmt.Sprintf("Kamaji version %s %s%s", internal.GitTag, internal.GitCommit, internal.GitDirty))
			setupLog.Info(fmt.Sprintf("Build from: %s", internal.GitRepo))
			setupLog.Info(fmt.Sprintf("Build date: %s", internal.BuildTime))
			setupLog.Info(fmt.Sprintf("Go Version: %s", goRuntime.Version()))
			setupLog.Info(fmt.Sprintf("Go OS/Arch: %s/%s", goRuntime.GOOS, goRuntime.GOARCH))

			ctrlOpts := ctrl.Options{
				Scheme: scheme,
				Metrics: metricsserver.Options{
					BindAddress: metricsBindAddress,
				},
				HealthProbeBindAddress:  healthProbeBindAddress,
				LeaderElection:          leaderElect,
				LeaderElectionNamespace: managerNamespace,
				LeaderElectionID:        "kubeconfiggenerator.kamaji.clastix.io",
				NewCache: func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
					opts.SyncPeriod = &cacheResyncPeriod

					return cache.New(config, opts)
				},
			}

			triggerChan := make(chan event.GenericEvent)

			mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrlOpts)
			if err != nil {
				setupLog.Error(err, "unable to start manager")

				return err
			}

			setupLog.Info("setting probes")
			{
				if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
					setupLog.Error(err, "unable to set up health check")

					return err
				}
				if err = mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
					setupLog.Error(err, "unable to set up ready check")

					return err
				}
			}

			certController := &controllers.CertificateLifecycle{Channel: triggerChan, Deadline: certificateExpirationDeadline}
			certController.EnqueueFn = certController.EnqueueForKubeconfigGenerator
			if err = certController.SetupWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "CertificateLifecycle")

				return err
			}

			if err = (&controllers.KubeconfigGeneratorWatcher{
				Client:        mgr.GetClient(),
				GeneratorChan: triggerChan,
			}).SetupWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "KubeconfigGeneratorWatcher")

				return err
			}

			if err = (&controllers.KubeconfigGeneratorReconciler{
				Client:            mgr.GetClient(),
				NotValidThreshold: certificateExpirationDeadline,
				CertificateChan:   triggerChan,
			}).SetupWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "KubeconfigGenerator")

				return err
			}

			setupLog.Info("starting manager")
			if err = mgr.Start(ctx); err != nil {
				setupLog.Error(err, "problem running manager")

				return err
			}

			return nil
		},
	}
	// Setting zap logger
	zapfs := flag.NewFlagSet("zap", flag.ExitOnError)
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(zapfs)
	cmd.Flags().AddGoFlagSet(zapfs)
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	// Setting CLI flags
	cmd.Flags().StringVar(&metricsBindAddress, "metrics-bind-address", ":8090", "The address the metric endpoint binds to.")
	cmd.Flags().StringVar(&healthProbeBindAddress, "health-probe-bind-address", ":8091", "The address the probe endpoint binds to.")
	cmd.Flags().BoolVar(&leaderElect, "leader-elect", true, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	cmd.Flags().DurationVar(&controllerReconcileTimeout, "controller-reconcile-timeout", 30*time.Second, "The reconciliation request timeout before the controller withdraw the external resource calls, such as dealing with the Datastore, or the Tenant Control Plane API endpoint.")
	cmd.Flags().DurationVar(&cacheResyncPeriod, "cache-resync-period", 10*time.Hour, "The controller-runtime.Manager cache resync period.")
	cmd.Flags().StringVar(&managerNamespace, "pod-namespace", os.Getenv("POD_NAMESPACE"), "The Kubernetes Namespace on which the Operator is running in, required for the TenantControlPlane migration jobs.")
	cmd.Flags().DurationVar(&certificateExpirationDeadline, "certificate-expiration-deadline", 24*time.Hour, "Define the deadline upon certificate expiration to start the renewal process, cannot be less than a 24 hours.")

	cobra.OnInitialize(func() {
		viper.AutomaticEnv()
	})

	return cmd
}
