// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/clastix/kamaji/cmd/kamaji"
	"github.com/clastix/kamaji/cmd/kamaji/manager"
	"github.com/clastix/kamaji/cmd/kamaji/migrate"
)

func main() {
	scheme := runtime.NewScheme()

	root, mgr, migrator := kamaji.NewCmd(scheme), manager.NewCmd(scheme), migrate.NewCmd(scheme)
	root.AddCommand(mgr)
	root.AddCommand(migrator)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
