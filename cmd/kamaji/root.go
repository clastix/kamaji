// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kamaji

import (
	"github.com/spf13/cobra"
	_ "go.uber.org/automaxprocs" // Automatically set `GOMAXPROCS` to match Linux container CPU quota.
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	appsv1 "k8s.io/kubernetes/pkg/apis/apps/v1"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func NewCmd(scheme *runtime.Scheme) *cobra.Command {
	return &cobra.Command{
		Use:   "kamaji",
		Short: "Build and operate Kubernetes at scale with a fraction of operational burden.",
		PersistentPreRun: func(*cobra.Command, []string) {
			utilruntime.Must(clientgoscheme.AddToScheme(scheme))
			utilruntime.Must(kamajiv1alpha1.AddToScheme(scheme))
			utilruntime.Must(appsv1.RegisterDefaults(scheme))
		},
	}
}
