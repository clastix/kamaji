// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"io"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/clastix/kamaji/cmd/kubectl-kamaji/utils"
	"github.com/clastix/kamaji/pkg/cli"
)

func NewTokenJoin(writer io.Writer, flags *genericclioptions.ConfigFlags, k8sClient client.Client) *cobra.Command {
	var flavour string
	var skipExpired bool

	cmd := cobra.Command{
		Use:     "join {TCP_NAME}",
		Example: "  kubectl [--namespace=$NAMESPACE] kamaji token join $TCP_NAME [--flavour=$FLAVOUR]",
		Short:   "Print join command",
		Long: "Prints the required command to launch on a worker node to let it join to a Tenant Control Plane.\n" +
			"\n" +
			"The CLI flag --flavour allows to generate the command according to the flavour, yaki, or standard kubeadm.\n" +
			"When the yaki flavour is selected, the environment variable KUBERNETES_VERSION will reference the current Kubernetes version: " +
			"you can customize it according to your needs, as well as expanding the available environment variables (reference: https://github.com/clastix/yaki)",
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utils.ValidArgsFunction(k8sClient, *flags.Namespace)(cmd, args, toComplete)
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := (&cli.Helper{Client: k8sClient}).JoinToken(cmd.Context(), *flags.Namespace, args[0], skipExpired, flavour)
			if err != nil {
				return err
			}

			_, _ = writer.Write([]byte(data))

			return nil
		},
	}

	cmd.Flags().StringVar(&flavour, "flavour", "yaki", "The flavour to use for the join command, supported values: yaki, kubeadm.")
	cmd.Flags().BoolVar(&skipExpired, "skip-expired", true, "When enabled, expired bootstrap tokens will be ignored.")

	return &cmd
}
