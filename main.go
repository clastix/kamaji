// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/clastix/kamaji/cmd"
	"github.com/clastix/kamaji/cmd/manager"
)

func main() {
	scheme := runtime.NewScheme()

	root, mgr := cmd.NewCmd(scheme), manager.NewCmd(scheme)
	root.AddCommand(mgr)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
