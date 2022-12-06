// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"

	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	deniedMessage = "the current Control Plane is in freezing mode due to a maintenance mode, all the changes are blocked: " +
		"removing the webhook may lead to an inconsistent state upon its completion"
)

type Freeze struct{}

func (f *Freeze) Handle(context.Context, admission.Request) admission.Response {
	return admission.Denied(deniedMessage)
}

func (f *Freeze) SetupWithManager(mgr controllerruntime.Manager) error {
	mgr.GetWebhookServer().Register("/migrate", &webhook.Admission{Handler: f})

	return nil
}
