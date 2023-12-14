// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	webhookhandlers "github.com/clastix/kamaji/internal/webhook/handlers"
	webhookroutes "github.com/clastix/kamaji/internal/webhook/routes"
)

func Register(mgr controllerruntime.Manager, routes map[webhookroutes.Route][]webhookhandlers.Handler) error {
	srv := mgr.GetWebhookServer()

	chainer := handlersChainer{
		decoder: admission.NewDecoder(mgr.GetScheme()),
	}

	for route, handlers := range routes {
		srv.Register(route.GetPath(), &webhook.Admission{
			Handler:      chainer.Handler(route.GetObject(), handlers...),
			RecoverPanic: true,
		})
	}

	return nil
}
