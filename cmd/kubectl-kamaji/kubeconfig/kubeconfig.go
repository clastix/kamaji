// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeconfig

import (
	"github.com/spf13/cobra"
)

func NewKubeconfigGroup() *cobra.Command {
	return &cobra.Command{
		Use:   "kubeconfig",
		Short: "kubeconfig operations",
		Long:  "Performs operations on kubeconfig objects related to the given Tenant Control Plane.",
	}
}
