// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	goRuntime "runtime"
	"time"

	telemetryclient "github.com/clastix/kamaji-telemetry/pkg/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	ctrlwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	cmdutils "github.com/clastix/kamaji/cmd/utils"
	"github.com/clastix/kamaji/controllers"
	"github.com/clastix/kamaji/controllers/soot"
	"github.com/clastix/kamaji/internal"
	"github.com/clastix/kamaji/internal/builders/controlplane"
	datastoreutils "github.com/clastix/kamaji/internal/datastore/utils"
	"github.com/clastix/kamaji/internal/utilities"
	"github.com/clastix/kamaji/internal/webhook"
	"github.com/clastix/kamaji/internal/webhook/handlers"
	"github.com/clastix/kamaji/internal/webhook/routes"
)

//nolint:maintidx
func NewCmd(scheme *runtime.Scheme) *cobra.Command {
	// CLI flags
	var (
		metricsBindAddress            string
		healthProbeBindAddress        string
		leaderElect                   bool
		tmpDirectory                  string
		kineImage                     string
		controllerReconcileTimeout    time.Duration
		cacheResyncPeriod             time.Duration
		datastore                     string
		managerNamespace              string
		managerServiceAccountName     string
		managerServiceName            string
		webhookCABundle               []byte
		migrateJobImage               string
		maxConcurrentReconciles       int
		disableTelemetry              bool
		certificateExpirationDeadline time.Duration

		webhookCAPath string
	)

	cmd := &cobra.Command{
		Use:           "manager",
		Short:         "Start the Kamaji Kubernetes Operator",
		SilenceErrors: false,
		SilenceUsage:  true,
		PreRunE: func(cmd *cobra.Command, _ []string) (err error) {
			// Avoid to pollute Kamaji stdout with useless details by the underlying klog implementations
			klog.SetOutput(io.Discard)
			klog.LogToStderr(false)

			if err = cmdutils.CheckFlags(cmd.Flags(), []string{"kine-image", "migrate-image", "tmp-directory", "pod-namespace", "webhook-service-name", "serviceaccount-name", "webhook-ca-path"}...); err != nil {
				return err
			}

			if certificateExpirationDeadline < 24*time.Hour {
				return fmt.Errorf("certificate expiration deadline must be at least 24 hours")
			}

			if webhookCABundle, err = os.ReadFile(webhookCAPath); err != nil {
				return fmt.Errorf("unable to read webhook CA: %w", err)
			}

			if err = datastoreutils.CheckExists(context.Background(), scheme, datastore); err != nil {
				return err
			}

			if controllerReconcileTimeout.Seconds() == 0 {
				return fmt.Errorf("the controller reconcile timeout must be greater than zero")
			}

			return nil
		},
		RunE: func(*cobra.Command, []string) error {
			ctx := ctrl.SetupSignalHandler()

			setupLog := ctrl.Log.WithName("setup")

			setupLog.Info(fmt.Sprintf("Kamaji version %s %s%s", internal.GitTag, internal.GitCommit, internal.GitDirty))
			setupLog.Info(fmt.Sprintf("Build from: %s", internal.GitRepo))
			setupLog.Info(fmt.Sprintf("Build date: %s", internal.BuildTime))
			setupLog.Info(fmt.Sprintf("Go Version: %s", goRuntime.Version()))
			setupLog.Info(fmt.Sprintf("Go OS/Arch: %s/%s", goRuntime.GOOS, goRuntime.GOARCH))
			setupLog.Info(fmt.Sprintf("Telemetry enabled: %t", !disableTelemetry))

			telemetryClient := telemetryclient.New(http.Client{Timeout: 5 * time.Second}, "https://telemetry.clastix.io")
			if disableTelemetry {
				telemetryClient = telemetryclient.NewNewOp()
			}

			ctrlOpts := ctrl.Options{
				Scheme: scheme,
				Metrics: metricsserver.Options{
					BindAddress: metricsBindAddress,
				},
				WebhookServer: ctrlwebhook.NewServer(ctrlwebhook.Options{
					Port: 9443,
				}),
				HealthProbeBindAddress:  healthProbeBindAddress,
				LeaderElection:          leaderElect,
				LeaderElectionNamespace: managerNamespace,
				LeaderElectionID:        "kamaji.clastix.io",
				NewCache: func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
					opts.SyncPeriod = &cacheResyncPeriod

					return cache.New(config, opts)
				},
			}

			mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrlOpts)
			if err != nil {
				setupLog.Error(err, "unable to start manager")

				return err
			}

			tcpChannel, certChannel := make(chan event.GenericEvent), make(chan event.GenericEvent)

			if err = (&controllers.DataStore{Client: mgr.GetClient(), TenantControlPlaneTrigger: tcpChannel}).SetupWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "DataStore")

				return err
			}

			discoveryClient, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())
			if err != nil {
				setupLog.Error(err, "unable to create discovery client")

				return err
			}

			reconciler := &controllers.TenantControlPlaneReconciler{
				Client:    mgr.GetClient(),
				APIReader: mgr.GetAPIReader(),
				Config: controllers.TenantControlPlaneReconcilerConfig{
					DefaultDataStoreName:    datastore,
					KineContainerImage:      kineImage,
					TmpBaseDirectory:        tmpDirectory,
					CertExpirationThreshold: certificateExpirationDeadline,
				},
				ReconcileTimeout:        controllerReconcileTimeout,
				CertificateChan:         certChannel,
				TriggerChan:             tcpChannel,
				KamajiNamespace:         managerNamespace,
				KamajiServiceAccount:    managerServiceAccountName,
				KamajiService:           managerServiceName,
				KamajiMigrateImage:      migrateJobImage,
				MaxConcurrentReconciles: maxConcurrentReconciles,
				DiscoveryClient:         discoveryClient,
			}

			if err = reconciler.SetupWithManager(ctx, mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "Namespace")

				return err
			}

			k8sVersion, versionErr := cmdutils.KubernetesVersion(mgr.GetConfig())
			if versionErr != nil {
				setupLog.Error(err, "unable to get kubernetes version")

				k8sVersion = "Unknown"
			}

			if !disableTelemetry {
				err = mgr.Add(&controllers.TelemetryController{
					Client:                  mgr.GetClient(),
					KubernetesVersion:       k8sVersion,
					KamajiVersion:           internal.GitTag,
					TelemetryClient:         telemetryClient,
					LeaderElectionNamespace: ctrlOpts.LeaderElectionNamespace,
					LeaderElectionID:        ctrlOpts.LeaderElectionID,
				})
				if err != nil {
					setupLog.Error(err, "unable to create controller", "controller", "TelemetryController")

					return err
				}
			}

			certController := &controllers.CertificateLifecycle{Channel: certChannel, Deadline: certificateExpirationDeadline}
			certController.EnqueueFn = certController.EnqueueForTenantControlPlane

			if err = certController.SetupWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "CertificateLifecycle")

				return err
			}

			if err = (&kamajiv1alpha1.DatastoreUsedSecret{}).SetupWithManager(ctx, mgr); err != nil {
				setupLog.Error(err, "unable to create indexer", "indexer", "DatastoreUsedSecret")

				return err
			}

			if err = (&kamajiv1alpha1.TenantControlPlaneStatusDataStore{}).SetupWithManager(ctx, mgr); err != nil {
				setupLog.Error(err, "unable to create indexer", "indexer", "TenantControlPlaneStatusDataStore")

				return err
			}

			// Only requires to look for the core api group.
			if utilities.AreGatewayResourcesAvailable(ctx, mgr.GetClient(), discoveryClient) {
				if err = (&kamajiv1alpha1.GatewayListener{}).SetupWithManager(ctx, mgr); err != nil {
					setupLog.Error(err, "unable to create indexer", "indexer", "GatewayListener")

					return err
				}
			}

			err = webhook.Register(mgr, map[routes.Route][]handlers.Handler{
				routes.TenantControlPlaneMigrate{}: {
					handlers.Freeze{},
				},
				routes.TenantControlPlaneWritePermission{}: {
					handlers.WritePermission{},
				},
				routes.TenantControlPlaneDefaults{}: {
					handlers.TenantControlPlaneDefaults{
						DefaultDatastore: datastore,
					},
				},
				routes.TenantControlPlaneValidate{}: {
					handlers.TenantControlPlaneCertSANs{},
					handlers.TenantControlPlaneName{},
					handlers.TenantControlPlaneVersion{},
					handlers.TenantControlPlaneDataStore{Client: mgr.GetClient()},
					handlers.TenantControlPlaneDeployment{
						Client: mgr.GetClient(),
						DeploymentBuilder: controlplane.Deployment{
							Client:             mgr.GetClient(),
							KineContainerImage: kineImage,
						},
						KonnectivityBuilder: controlplane.Konnectivity{
							Scheme: *mgr.GetScheme(),
						},
					},
					handlers.TenantControlPlaneServiceCIDR{},
					handlers.TenantControlPlaneLoadBalancerSourceRanges{},
					handlers.TenantControlPlaneGatewayValidation{
						Client:          mgr.GetClient(),
						DiscoveryClient: discoveryClient,
					},
				},
				routes.TenantControlPlaneTelemetry{}: {
					handlers.TenantControlPlaneTelemetry{
						Enabled:           !disableTelemetry,
						TelemetryClient:   telemetryClient,
						KamajiVersion:     internal.GitTag,
						KubernetesVersion: k8sVersion,
					},
				},
				routes.DataStoreValidate{}: {
					handlers.DataStoreValidation{Client: mgr.GetClient()},
				},
				routes.DataStoreSecrets{}: {
					handlers.DataStoreSecretValidation{Client: mgr.GetClient()},
				},
			})
			if err != nil {
				setupLog.Error(err, "unable to create webhook")

				return err
			}

			if err = (&soot.Manager{
				MigrateCABundle:         webhookCABundle,
				MigrateServiceName:      managerServiceName,
				MigrateServiceNamespace: managerNamespace,
				AdminClient:             mgr.GetClient(),
			}).SetupWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to set up soot manager")

				return err
			}

			if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
				setupLog.Error(err, "unable to set up health check")

				return err
			}
			if err = mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
				setupLog.Error(err, "unable to set up ready check")

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
	cmd.Flags().StringVar(&metricsBindAddress, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	cmd.Flags().StringVar(&healthProbeBindAddress, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	cmd.Flags().BoolVar(&leaderElect, "leader-elect", true, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	cmd.Flags().StringVar(&tmpDirectory, "tmp-directory", "/tmp/kamaji", "Directory which will be used to work with temporary files.")
	cmd.Flags().StringVar(&kineImage, "kine-image", "rancher/kine:v0.11.10-amd64", "Container image along with tag to use for the Kine sidecar container (used only if etcd-storage-type is set to one of kine strategies).")
	cmd.Flags().StringVar(&datastore, "datastore", "", "Optional, the default DataStore that should be used by Kamaji to setup the required storage of Tenant Control Planes with undeclared DataStore.")
	cmd.Flags().StringVar(&migrateJobImage, "migrate-image", fmt.Sprintf("%s/clastix/kamaji:%s", internal.ContainerRepository, internal.GitTag), "Specify the container image to launch when a TenantControlPlane is migrated to a new datastore.")
	cmd.Flags().IntVar(&maxConcurrentReconciles, "max-concurrent-tcp-reconciles", 1, "Specify the number of workers for the Tenant Control Plane controller (beware of CPU consumption)")
	cmd.Flags().StringVar(&managerNamespace, "pod-namespace", os.Getenv("POD_NAMESPACE"), "The Kubernetes Namespace on which the Operator is running in, required for the TenantControlPlane migration jobs.")
	cmd.Flags().StringVar(&managerServiceName, "webhook-service-name", "kamaji-webhook-service", "The Kamaji webhook server Service name which is used to get validation webhooks, required for the TenantControlPlane migration jobs.")
	cmd.Flags().StringVar(&managerServiceAccountName, "serviceaccount-name", os.Getenv("SERVICE_ACCOUNT"), "The Kubernetes Namespace on which the Operator is running in, required for the TenantControlPlane migration jobs.")
	cmd.Flags().StringVar(&webhookCAPath, "webhook-ca-path", "/tmp/k8s-webhook-server/serving-certs/ca.crt", "Path to the Manager webhook server CA, required for the TenantControlPlane migration jobs.")
	cmd.Flags().DurationVar(&controllerReconcileTimeout, "controller-reconcile-timeout", 30*time.Second, "The reconciliation request timeout before the controller withdraw the external resource calls, such as dealing with the Datastore, or the Tenant Control Plane API endpoint.")
	cmd.Flags().DurationVar(&cacheResyncPeriod, "cache-resync-period", 10*time.Hour, "The controller-runtime.Manager cache resync period.")
	cmd.Flags().BoolVar(&disableTelemetry, "disable-telemetry", false, "Disable the analytics traces collection.")
	cmd.Flags().DurationVar(&certificateExpirationDeadline, "certificate-expiration-deadline", 24*time.Hour, "Define the deadline upon certificate expiration to start the renewal process, cannot be less than a 24 hours.")

	cobra.OnInitialize(func() {
		viper.AutomaticEnv()
	})

	return cmd
}
