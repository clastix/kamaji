// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package certificates

import (
	"github.com/spf13/cobra"
)

func NewCertificatesGroup() *cobra.Command {
	return &cobra.Command{
		Use:   "certificates",
		Short: "Certificate operations",
		Long:  "Performs operations on Tenant Control Plance related certificates.",
	}
}
