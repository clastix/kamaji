// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"testing"
	"time"
)

func TestWithShutdownDrain(t *testing.T) {
	const drain = 300 * time.Millisecond

	parent, stop := context.WithCancel(context.Background())
	ctx := withShutdownDrain(parent, drain)

	select {
	case <-ctx.Done():
		t.Fatal("context is done before the parent is")
	case <-time.After(50 * time.Millisecond):
	}

	stop()

	select {
	case <-ctx.Done():
		t.Fatal("context is done before the drain period has elapsed")
	case <-time.After(100 * time.Millisecond):
	}

	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("context is not done after the drain period")
	}
}
