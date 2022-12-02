// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/clastix/kamaji/cmd"
	"github.com/clastix/kamaji/cmd/manager"
	"github.com/clastix/kamaji/cmd/migrate"
)

func main() {
	scheme := runtime.NewScheme()

	root, mgr, migrator := cmd.NewCmd(scheme), manager.NewCmd(scheme), migrate.NewCmd(scheme)
	root.AddCommand(mgr)
	root.AddCommand(migrator)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
