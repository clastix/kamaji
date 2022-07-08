// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package api

type KubeadmConfigResourceVersionDependant interface {
	GetKubeadmConfigResourceVersion() string
	SetKubeadmConfigResourceVersion(string)
}
