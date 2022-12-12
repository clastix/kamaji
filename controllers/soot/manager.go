// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package soot

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/controllers/soot/controllers"
	"github.com/clastix/kamaji/controllers/utils"
	"github.com/clastix/kamaji/internal/utilities"
)

type sootItem struct {
	triggers []chan event.GenericEvent
	cancelFn context.CancelFunc
}

type sootMap map[string]sootItem

type Manager struct {
	client  client.Client
	sootMap sootMap

	MigrateCABundle         []byte
	MigrateServiceName      string
	MigrateServiceNamespace string
	AdminClient             client.Client
}

// retrieveTenantControlPlane is the function used to let an underlying controller of the soot manager
// to retrieve its parent TenantControlPlane definition, required to understand which actions must be performed.
func (m *Manager) retrieveTenantControlPlane(ctx context.Context, request reconcile.Request) utils.TenantControlPlaneRetrievalFn {
	return func() (*kamajiv1alpha1.TenantControlPlane, error) {
		tcp := &kamajiv1alpha1.TenantControlPlane{}

		if err := m.client.Get(ctx, request.NamespacedName, tcp); err != nil {
			return nil, err
		}

		return tcp, nil
	}
}

// If the TenantControlPlane is deleted we have to free up memory by stopping the soot manager:
// this is made possible by retrieving the cancel function of the soot manager context to cancel it.
func (m *Manager) cleanup(req reconcile.Request) error { //nolint:unparam
	tcpName := req.NamespacedName.String()

	v, ok := m.sootMap[tcpName]
	if !ok {
		return nil
	}

	v.cancelFn()

	delete(m.sootMap, tcpName)

	return nil
}

func (m *Manager) Reconcile(ctx context.Context, request reconcile.Request) (res reconcile.Result, err error) {
	// Retrieving the TenantControlPlane:
	// in case of deletion, we must be sure to properly remove from the memory the soot manager.
	tcp := &kamajiv1alpha1.TenantControlPlane{}
	if err = m.client.Get(ctx, request.NamespacedName, tcp); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, m.cleanup(request)
		}

		return reconcile.Result{}, err
	}

	tcpStatus := *tcp.Status.Kubernetes.Version.Status
	// Triggering the reconciliation of the underlying controllers of
	// the soot manager if this is already registered.
	v, ok := m.sootMap[request.String()]
	if ok {
		// The TenantControlPlane is in non-ready mode:
		// we don't want to pollute with messages due to broken connection.
		// Once the TCP will be ready again, the event will be intercepted and the manager started back.
		if tcpStatus == kamajiv1alpha1.VersionNotReady {
			v.cancelFn()

			return reconcile.Result{}, nil
		}

		for _, trigger := range v.triggers {
			trigger <- event.GenericEvent{Object: tcp}
		}

		return reconcile.Result{}, nil
	}
	// No need to start a soot manager if the TenantControlPlane is not ready:
	// enqueuing back is not required since we're going to get that event once ready.
	if tcpStatus == kamajiv1alpha1.VersionNotReady {
		log.FromContext(ctx).Info("skipping start of the soot manager for a not ready instance")

		return reconcile.Result{}, nil
	}
	// Generating the manager and starting it:
	// in case of any error, reconciling the request to start it back from the beginning.
	tcpRest, err := utilities.GetRESTClientConfig(ctx, m.client, tcp)
	if err != nil {
		return reconcile.Result{}, err
	}

	tcpCtx, tcpCancelFn := context.WithCancel(ctx)
	defer func() {
		// If the reconciliation fails, we don't need to get a potential dangling goroutine.
		if err != nil {
			tcpCancelFn()
		}
	}()

	mgr, err := controllerruntime.NewManager(tcpRest, controllerruntime.Options{
		Logger:             log.Log.WithName(fmt.Sprintf("soot_%s_%s", tcp.GetNamespace(), tcp.GetName())),
		Scheme:             m.client.Scheme(),
		MetricsBindAddress: "0",
		NewClient: func(cache cache.Cache, config *rest.Config, options client.Options, uncachedObjects ...client.Object) (client.Client, error) {
			return client.New(config, client.Options{
				Scheme: m.client.Scheme(),
			})
		},
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	//
	// Register all the controllers of the soot here:
	//
	migrate := &controllers.Migrate{
		WebhookNamespace:          m.MigrateServiceName,
		WebhookServiceName:        m.MigrateServiceNamespace,
		WebhookCABundle:           m.MigrateCABundle,
		GetTenantControlPlaneFunc: m.retrieveTenantControlPlane(tcpCtx, request),
		TriggerChannel:            make(chan event.GenericEvent),
	}
	if err = migrate.SetupWithManager(mgr); err != nil {
		return reconcile.Result{}, err
	}

	konnectivityAgent := &controllers.KonnectivityAgent{
		AdminClient:               m.AdminClient,
		GetTenantControlPlaneFunc: m.retrieveTenantControlPlane(tcpCtx, request),
		TriggerChannel:            make(chan event.GenericEvent),
	}
	if err = konnectivityAgent.SetupWithManager(mgr); err != nil {
		return reconcile.Result{}, err
	}
	// Starting the manager
	go func() {
		if err = mgr.Start(tcpCtx); err != nil {
			log.FromContext(ctx).Error(err, "unable to start soot manager")
		}
	}()

	m.sootMap[request.NamespacedName.String()] = sootItem{
		triggers: []chan event.GenericEvent{
			migrate.TriggerChannel,
			konnectivityAgent.TriggerChannel,
		},
		cancelFn: tcpCancelFn,
	}

	return reconcile.Result{Requeue: true}, nil
}

func (m *Manager) SetupWithManager(mgr manager.Manager) error {
	m.client = mgr.GetClient()
	m.sootMap = make(map[string]sootItem)

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&kamajiv1alpha1.TenantControlPlane{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			obj := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert
			// status is required to understand if we have to start or stop the soot manager
			if obj.Status.Kubernetes.Version.Status == nil {
				return false
			}

			if *obj.Status.Kubernetes.Version.Status == kamajiv1alpha1.VersionProvisioning {
				return false
			}

			return true
		}))).
		Complete(m)
}
