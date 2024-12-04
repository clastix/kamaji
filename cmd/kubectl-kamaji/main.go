// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/cmd/kubectl-kamaji/certificates"
	"github.com/clastix/kamaji/cmd/kubectl-kamaji/kubeconfig"
	"github.com/clastix/kamaji/cmd/kubectl-kamaji/token"
)

func main() {
	writer := bufio.NewWriter(os.Stdout)
	scheme := runtime.NewScheme()

	configFlags := genericclioptions.NewConfigFlags(true)

	clientConfig := configFlags.ToRawKubeConfigLoader()
	*configFlags.Namespace, _, _ = clientConfig.Namespace()

	restConfig, restErr := clientConfig.ClientConfig()
	if restErr != nil {
		_, _ = writer.WriteString(fmt.Sprintf("cannot retrieve REST configuration: %s", restErr.Error()))
		os.Exit(1)
	}

	k8sClient, clientErr := client.New(restConfig, client.Options{Scheme: scheme})
	if clientErr != nil {
		_, _ = writer.WriteString(fmt.Sprintf("cannot generate Kubernetes configuration: %s", clientErr))
		os.Exit(1)
	}

	rootCmd := NewCmd(scheme, configFlags)

	kubeconfigCmd := kubeconfig.NewKubeconfigGroup()
	kubeconfigCmd.AddCommand(kubeconfig.NewGetKubeconfig(writer, configFlags, k8sClient))
	kubeconfigCmd.AddCommand(kubeconfig.NewRotateKubeconfig(writer, configFlags, k8sClient))
	rootCmd.AddCommand(kubeconfigCmd)

	certificatesCmd := certificates.NewCertificatesGroup()
	certificatesCmd.AddCommand(certificates.NewRotateCertificates(writer, configFlags, k8sClient))
	rootCmd.AddCommand(certificatesCmd)

	tokenCmd := token.NewTokenGroup()
	tokenCmd.AddCommand(token.NewTokenJoin(writer, configFlags, k8sClient))
	rootCmd.AddCommand(tokenCmd)

	ctx, cancelFn := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFn()

	if _, err := rootCmd.ExecuteContextC(ctx); err != nil {
		cancelFn()
		os.Exit(1) //nolint:gocritic
	}

	_ = writer.Flush()
}

func NewCmd(scheme *runtime.Scheme, flags *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := cobra.Command{
		Use:     "kubectl-kamaji",
		Aliases: []string{"kubectl kamaji"},
		Short:   "A plugin to manage your Kamaji Tenant Control Planes with ease.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			utilruntime.Must(clientgoscheme.AddToScheme(scheme))
			utilruntime.Must(kamajiv1alpha1.AddToScheme(scheme))
		},
	}

	flags.AddFlags(cmd.PersistentFlags())

	return &cmd
}
