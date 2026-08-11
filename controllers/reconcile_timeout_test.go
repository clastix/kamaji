// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// contextCapturingClient wraps a client.Client and records the deadline of the context passed to Get,
// so tests can assert a reconciler actually derives its client calls from ReconcileTimeout.
type contextCapturingClient struct {
	client.Client

	capturedDeadline time.Time
	hasDeadline      bool
}

func (c *contextCapturingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	c.capturedDeadline, c.hasDeadline = ctx.Deadline()

	return c.Client.Get(ctx, key, obj, opts...)
}

func assertContextDeadlineWithin(t *testing.T, deadline time.Time, hasDeadline bool, want time.Duration) {
	t.Helper()

	if !hasDeadline {
		t.Fatalf("expected the reconciler to have issued a Get call with a context carrying a deadline derived from ReconcileTimeout")
	}

	if remaining := time.Until(deadline); remaining <= 0 || remaining > want {
		t.Fatalf("expected context deadline within (0, %s], got %s", want, remaining)
	}
}
