// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/clastix/kamaji/internal/constants"
	"github.com/clastix/kamaji/internal/utilities"
)

func setKamajiManagedLabels(obj client.Object) {
	obj.SetLabels(utilities.MergeMaps(obj.GetLabels(), map[string]string{
		constants.ProjectNameLabelKey: constants.ProjectNameLabelValue,
	}))
}
