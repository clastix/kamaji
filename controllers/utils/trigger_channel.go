// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

const (
	TriggerChannelBufferSize = 10
	TriggerChannelTimeout    = 30 * time.Second
)

func TriggerChannel(ctx context.Context, receiver chan event.GenericEvent, tcp kamajiv1alpha1.TenantControlPlane) {
	deadlineCtx, cancelFn := context.WithTimeout(ctx, TriggerChannelTimeout)
	defer cancelFn()

	select {
	case receiver <- event.GenericEvent{Object: &tcp}:
		return
	case <-deadlineCtx.Done():
		log.FromContext(ctx).Error(deadlineCtx.Err(), "cannot send due to timeout")
	}
}
