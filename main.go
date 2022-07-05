/*
Copyright 2021.

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
	"fmt"
	"log"
	"os"
	goRuntime "runtime"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/controllers"
	"github.com/clastix/kamaji/internal"
	"github.com/clastix/kamaji/internal/config"
	"github.com/clastix/kamaji/internal/types"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(kamajiv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
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

	reconciler := &controllers.TenantControlPlaneReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Config: controllers.TenantControlPlaneReconcilerConfig{
			ETCDStorageType:           types.ParseETCDStorageType(conf.GetString("etcd-storage-type")),
			ETCDCASecretName:          conf.GetString("etcd-ca-secret-name"),
			ETCDCASecretNamespace:     conf.GetString("etcd-ca-secret-namespace"),
			ETCDClientSecretName:      conf.GetString("etcd-client-secret-name"),
			ETCDClientSecretNamespace: conf.GetString("etcd-client-secret-namespace"),
			ETCDEndpoints:             types.ParseETCDEndpoint(conf),
			ETCDCompactionInterval:    conf.GetString("etcd-compaction-interval"),
			TmpBaseDirectory:          conf.GetString("tmp-directory"),
			KineMySQLSecretName:       conf.GetString("kine-mysql-secret-name"),
			KineMySQLSecretNamespace:  conf.GetString("kine-mysql-secret-namespace"),
			KineMySQLHost:             conf.GetString("kine-mysql-host"),
			KineMySQLPort:             conf.GetInt("kine-mysql-port"),
		},
	}

	if err := reconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Namespace")
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
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
