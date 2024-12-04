// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package certificates

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/clastix/kamaji/cmd/kubectl-kamaji/utils"
	"github.com/clastix/kamaji/pkg/cli"
)

func NewRotateCertificates(writer io.Writer, flags *genericclioptions.ConfigFlags, k8sClient client.Client) *cobra.Command {
	var all bool
	var certificates []string

	cmd := cobra.Command{
		Use: "rotate {TCP_NAME} { --all | --certificates={CERTS_LIST} }",
		Example: "  kubectl [--namespace=$NAMESPACE] kamaji certificates rotate $TCP_NAME --all=true\n" +
			"  kubectl [--namespace=$NAMESPACE] kamaji certificates rotate $TCP_NAME --certificates=APIServer\n" +
			"  kubectl [--namespace=$NAMESPACE] kamaji certificates rotate $TCP_NAME --certificates=FrontProxyCA,FrontProxyClient",
		Short: "Get kubeconfig",
		Long: "Rotate one ore more Tenant Control Plane certificates.\n" +
			"\n" +
			"The CLI flag --certificates allows to specify which certificates kind should be rotated, such as: " +
			strings.Join(cli.RotateCertificatesMap.Keys(), ", ") + ".\n" +
			"At least one must be specified, or mutually exclusive with --all",
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utils.ValidArgsFunction(k8sClient, *flags.Namespace)(cmd, args, toComplete)
		},
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case all:
				return nil
			case len(certificates) == 0:
				return fmt.Errorf("at least one certificate must be specified")
			default:
				for _, arg := range certificates {
					if _, ok := cli.RotateCertificatesMap[arg]; !ok {
						return fmt.Errorf("unrecognized certificate, %q", arg)
					}
				}

				return nil
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			certSet := sets.New[string](certificates...)

			certsToDelete := make(cli.RotateCertOptions)
			for k, v := range cli.RotateCertificatesMap {
				if all || certSet.Has(k) {
					certsToDelete[k] = v
				}
			}

			if err := (&cli.Helper{Client: k8sClient}).RotateCertificate(cmd.Context(), *flags.Namespace, args[0], certsToDelete); err != nil {
				return err
			}

			_, _ = writer.Write([]byte(fmt.Sprintf("The following certificates have been successfully rotated: %s", strings.Join(certsToDelete.Keys(), ","))))

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&certificates, "certificates", []string{}, "Specify which certificates should be rotated, at least one should be provided.")
	cmd.Flags().BoolVar(&all, "all", false, "When specified, rotate all available certificates related to this Tenant Control Plane.")

	return &cmd
}
