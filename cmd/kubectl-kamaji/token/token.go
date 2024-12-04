// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"github.com/spf13/cobra"
)

func NewTokenGroup() *cobra.Command {
	return &cobra.Command{
		Use:   "token",
		Short: "Bootstrap token operations",
		Long:  "Creates the required join token commands to let a Worker Node join the given Tenant Control Plane.",
	}
}
