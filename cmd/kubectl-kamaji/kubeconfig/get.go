// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeconfig

import (
	"io"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/clastix/kamaji/cmd/kubectl-kamaji/utils"
	"github.com/clastix/kamaji/pkg/cli"
)

func NewGetKubeconfig(writer io.Writer, flags *genericclioptions.ConfigFlags, k8sClient client.Client) *cobra.Command {
	var secretKey string

	cmd := cobra.Command{
		Use:     "get {TCP_NAME}",
		Example: "  kubectl [--namespace=$NAMESPACE] kamaji kubeconfig get $TCP_NAME [--secret-type=$KEY]",
		Short:   "Get kubeconfig",
		Long: "Retrieve the decoded content of a Tenant Control Plane kubeconfig.\n" +
			"\n" +
			"The CLI flag --secret-type refers to the key name of the kubeconfig you want to extract. " +
			"By default, the command will extract the `admin.conf` one, you can specify your preferred one until it exists.",
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utils.ValidArgsFunction(k8sClient, *flags.Namespace)(cmd, args, toComplete)
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, data, kcErr := (&cli.Helper{Client: k8sClient}).GetKubeconfig(cmd.Context(), *flags.Namespace, args[0], secretKey)
			if kcErr != nil {
				return kcErr
			}

			_, _ = writer.Write(data)

			return nil
		},
	}

	cmd.Flags().StringVar(&secretKey, "secret-key", "admin.conf", "The Secret key used to retrieve the kubeconfig.")

	return &cmd
}
