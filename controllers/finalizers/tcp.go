// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package finalizers

const (
	// DatastoreFinalizer is using a wrong name, since it's related to the underlying datastore.
	DatastoreFinalizer = "finalizer.kamaji.clastix.io"
	SootFinalizer      = "finalizer.kamaji.clastix.io/soot"
)
