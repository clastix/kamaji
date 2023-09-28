// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package finalizers

const (
	// DatastoreFinalizer is using a wrong name, since it's related to the underlying datastore.
	DatastoreFinalizer       = "finalizer.kamaji.clastix.io"
	DatastoreSecretFinalizer = "finalizer.kamaji.clastix.io/datastore-secret"
	SootFinalizer            = "finalizer.kamaji.clastix.io/soot"
)
