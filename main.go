// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/clastix/kamaji/cmd"
	kubeconfig_generator "github.com/clastix/kamaji/cmd/kubeconfig-generator"
	"github.com/clastix/kamaji/cmd/manager"
	"github.com/clastix/kamaji/cmd/migrate"
)

func main() {
	scheme := runtime.NewScheme()

	root, mgr, migrator, kubeconfigGenerator := cmd.NewCmd(scheme), manager.NewCmd(scheme), migrate.NewCmd(scheme), kubeconfig_generator.NewCmd(scheme)
	root.AddCommand(mgr)
	root.AddCommand(migrator)
	root.AddCommand(kubeconfigGenerator)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
