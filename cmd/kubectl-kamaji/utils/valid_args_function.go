// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/clastix/kamaji/api/v1alpha1"
)

func ValidArgsFunction(k8sClient client.Client, namespace string) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveError
		}

		var tcpList v1alpha1.TenantControlPlaneList
		if err := k8sClient.List(cmd.Context(), &tcpList, client.InNamespace(namespace)); err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		var results []string

		for _, tcp := range tcpList.Items {
			if strings.HasPrefix(tcp.Name, toComplete) {
				results = append(results, tcp.Name)
			}
		}

		return results, cobra.ShellCompDirectiveNoSpace
	}
}
