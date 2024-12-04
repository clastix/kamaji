// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"

	"github.com/spf13/pflag"
)

func CheckFlags(flags *pflag.FlagSet, args ...string) error {
	for _, arg := range args {
		v, _ := flags.GetString(arg)

		if len(v) == 0 {
			return fmt.Errorf("expecting a value for --%s arg", arg)
		}
	}

	return nil
}
