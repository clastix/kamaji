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

func TriggerChannel(ctx context.Context, receiver chan event.GenericEvent, tcp kamajiv1alpha1.TenantControlPlane) {
	deadlineCtx, cancelFn := context.WithTimeout(ctx, 10*time.Second)
	defer cancelFn()

	select {
	case receiver <- event.GenericEvent{Object: &tcp}:
		return
	case <-deadlineCtx.Done():
		log.FromContext(ctx).Error(deadlineCtx.Err(), "cannot send due to timeout")
	}
}
