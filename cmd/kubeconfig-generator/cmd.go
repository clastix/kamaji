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
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	cmdutils "github.com/clastix/kamaji/cmd/utils"
	"github.com/clastix/kamaji/controllers"
	"github.com/clastix/kamaji/internal"
	kamajimanager "github.com/clastix/kamaji/internal/manager"
	"github.com/clastix/kamaji/internal/metrics"
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
		watchNamespaces               []string
	)

	cmd := &cobra.Command{
		Use:           "kubeconfig-generator",
		Short:         "Start the Kubeconfig Generator manager",
		SilenceErrors: false,
		SilenceUsage:  true,
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			// Avoid polluting stdout with useless details by the underlying klog implementations
			klog.SetOutput(io.Discard)
			klog.LogToStderr(false)

			// pod-namespace is required: the operator merges it into the
			// cache watch set when --watch-namespaces is non-empty (so
			// leader-election Lease access stays in scope), and the
			// migration Job watch needs it as well. The chart projects
			// it from metadata.namespace; binary-direct callers must
			// pass --pod-namespace=$NS or set POD_NAMESPACE.
			err := cmdutils.CheckFlags(cmd.Flags(), "pod-namespace")
			if err != nil {
				return err
			}

			if certificateExpirationDeadline < 24*time.Hour {
				return fmt.Errorf("certificate expiration deadline must be at least 24 hours")
			}

			err = kamajimanager.ValidateNamespaces(watchNamespaces)
			if err != nil {
				return err
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

			// kubeconfig-generator watches the cluster-scoped KubeconfigGenerator
			// CRD plus TenantControlPlane resources and labelled kubeconfig
			// Secrets in tenant namespaces. The install namespace is included
			// for symmetry with the main controller and to keep
			// leader-election working defensively if controller-runtime ever
			// routes the Lease informer through the manager cache.
			cachedNamespaces := kamajimanager.MergeWatchedNamespaces(watchNamespaces, managerNamespace)

			if len(cachedNamespaces) > 0 {
				setupLog.Info("restricting cache to namespaces", "namespaces", cachedNamespaces)
			}

			ctrlOpts := ctrl.Options{
				Scheme: scheme,
				Metrics: metricsserver.Options{
					BindAddress: metricsBindAddress,
				},
				HealthProbeBindAddress:  healthProbeBindAddress,
				LeaderElection:          leaderElect,
				LeaderElectionNamespace: managerNamespace,
				LeaderElectionID:        "kubeconfiggenerator.kamaji.clastix.io",
				NewCache:                kamajimanager.NewCacheFunc(cacheResyncPeriod, cachedNamespaces),
			}

			triggerChan := make(chan event.GenericEvent)
			metricsRecorder := metrics.DefaultRecorder()

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

			certController := &controllers.CertificateLifecycle{Channel: triggerChan, Deadline: certificateExpirationDeadline, Metrics: metricsRecorder}
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
	cmd.Flags().StringSliceVar(&watchNamespaces, "watch-namespaces", nil, "Optional, comma-separated list of namespaces the controller should watch for TenantControlPlane (and dependent) resources. When empty every namespace is watched. Cluster-scoped resources are never affected by this flag, and the install namespace is always watched implicitly.")

	cobra.OnInitialize(func() {
		viper.AutomaticEnv()
	})

	return cmd
}
