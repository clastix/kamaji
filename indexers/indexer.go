// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package indexers

import "sigs.k8s.io/controller-runtime/pkg/client"

type Indexer interface {
	Object() client.Object
	Field() string
	ExtractValue() client.IndexerFunc
}
