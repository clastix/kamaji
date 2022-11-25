// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	goRuntime "runtime"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/controllers"
	"github.com/clastix/kamaji/indexers"
	"github.com/clastix/kamaji/internal"
	"github.com/clastix/kamaji/internal/config"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	// Seed is required to ensure non reproducibility for the certificates generate by Kamaji.
	rand.Seed(time.Now().UnixNano())
	// Avoid to pollute Kamaji stdout with useless details by the underlying klog implementations
	klog.SetOutput(io.Discard)
	klog.LogToStderr(false)

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(kamajiv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	ctx := ctrl.SetupSignalHandler()

	conf, err := config.InitConfig()
	if err != nil {
		log.Fatalf("Error reading configuration.")
	}

	setupLog.Info(fmt.Sprintf("Kamaji version %s %s%s", internal.GitTag, internal.GitCommit, internal.GitDirty))
	setupLog.Info(fmt.Sprintf("Build from: %s", internal.GitRepo))
	setupLog.Info(fmt.Sprintf("Build date: %s", internal.BuildTime))
	setupLog.Info(fmt.Sprintf("Go Version: %s", goRuntime.Version()))
	setupLog.Info(fmt.Sprintf("Go OS/Arch: %s/%s", goRuntime.GOOS, goRuntime.GOARCH))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     conf.GetString("metrics-bind-address"),
		Port:                   9443,
		HealthProbeBindAddress: conf.GetString("health-probe-bind-address"),
		LeaderElection:         conf.GetBool("leader-elect"),
		LeaderElectionID:       "799b98bc.clastix.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	tcpChannel := make(controllers.TenantControlPlaneChannel)

	if err = (&controllers.DataStore{TenantControlPlaneTrigger: tcpChannel}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DataStore")
		os.Exit(1)
	}

	reconciler := &controllers.TenantControlPlaneReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Config: controllers.TenantControlPlaneReconcilerConfig{
			DefaultDataStoreName: conf.GetString("datastore"),
			KineContainerImage:   conf.GetString("kine-image"),
			TmpBaseDirectory:     conf.GetString("tmp-directory"),
		},
		TriggerChan: tcpChannel,
	}

	if err := reconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Namespace")
		os.Exit(1)
	}

	if err = (&indexers.TenantControlPlaneStatusDataStore{}).SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create indexer", "indexer", "TenantControlPlaneStatusDataStore")
		os.Exit(1)
	}

	if err = (&kamajiv1alpha1.TenantControlPlane{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "TenantControlPlane")
		os.Exit(1)
	}
	if err = (&kamajiv1alpha1.DataStore{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "DataStore")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
