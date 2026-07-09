// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"slices"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/controllers/utils"
)

type DataStoreUsage struct {
	Client client.Client
}

//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=datastores,verbs=get;list;watch
//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=datastores/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=get;list;watch

func (r *DataStoreUsage) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)

	var ds kamajiv1alpha1.DataStore
	if err := r.Client.Get(ctx, request.NamespacedName, &ds); err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Info("resource may have been deleted, skipping")

			return reconcile.Result{}, nil
		}

		logger.Error(err, "cannot retrieve the required resource")

		return reconcile.Result{}, err
	}

	if utils.IsPaused(&ds) {
		logger.Info("paused reconciliation, no further actions")

		return reconcile.Result{}, nil
	}

	tcpList, err := listTenantControlPlanesUsingDataStore(ctx, r.Client, ds.GetName())
	if err != nil {
		logger.Error(err, "cannot retrieve list of the Tenant Control Plane using the following instance")

		return reconcile.Result{}, err
	}

	usedBy := tenantControlPlaneNamespacedNames(tcpList)
	if equalStringSet(ds.Status.UsedBy, usedBy) {
		return reconcile.Result{}, nil
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var latest kamajiv1alpha1.DataStore
		if getErr := r.Client.Get(ctx, request.NamespacedName, &latest); getErr != nil {
			return getErr
		}

		if equalStringSet(latest.Status.UsedBy, usedBy) {
			return nil
		}

		base := latest.DeepCopy()
		latest.Status.UsedBy = usedBy

		return r.Client.Status().Patch(ctx, &latest, client.MergeFrom(base))
	})
	if err != nil {
		logger.Error(err, "cannot update usedBy status for the given instance")

		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *DataStoreUsage) SetupWithManager(mgr controllerruntime.Manager) error {
	enqueueFn := func(dataStoreName string, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
		if len(dataStoreName) == 0 {
			return
		}

		q.AddRateLimited(reconcile.Request{
			NamespacedName: k8stypes.NamespacedName{
				Name: dataStoreName,
			},
		})
	}
	//nolint:forcetypeassert
	return controllerruntime.NewControllerManagedBy(mgr).
		Named("datastoreusage").
		For(&kamajiv1alpha1.DataStore{}, builder.WithPredicates(dataStoreUsagePredicate{})).
		Watches(&kamajiv1alpha1.TenantControlPlane{}, handler.Funcs{
			CreateFunc: func(_ context.Context, createEvent event.TypedCreateEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				tcp := createEvent.Object.(*kamajiv1alpha1.TenantControlPlane)
				enqueueFn(tcp.Status.Storage.DataStoreName, q)
			},
			UpdateFunc: func(_ context.Context, updateEvent event.TypedUpdateEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				oldTCP := updateEvent.ObjectOld.(*kamajiv1alpha1.TenantControlPlane)
				newTCP := updateEvent.ObjectNew.(*kamajiv1alpha1.TenantControlPlane)

				if !shouldEnqueueDataStoreUsageUpdate(oldTCP, newTCP) {
					return
				}

				enqueueFn(oldTCP.Status.Storage.DataStoreName, q)
				enqueueFn(newTCP.Status.Storage.DataStoreName, q)
			},
			DeleteFunc: func(_ context.Context, deleteEvent event.TypedDeleteEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				tcp := deleteEvent.Object.(*kamajiv1alpha1.TenantControlPlane)
				enqueueFn(tcp.Status.Storage.DataStoreName, q)
			},
		}).
		Complete(r)
}

// dataStoreUsagePredicate reconciles usage when a DataStore appears. Updates
// are ignored because DataStoreUsage patches DataStore.status.usedBy itself.
// Deletes are ignored because there is no remaining status to maintain.
type dataStoreUsagePredicate struct {
	predicate.Funcs
}

func (dataStoreUsagePredicate) Create(event.TypedCreateEvent[client.Object]) bool {
	return true
}

func (dataStoreUsagePredicate) Delete(event.TypedDeleteEvent[client.Object]) bool {
	return false
}

func (dataStoreUsagePredicate) Update(event.TypedUpdateEvent[client.Object]) bool {
	return false
}

func (dataStoreUsagePredicate) Generic(event.TypedGenericEvent[client.Object]) bool {
	return false
}

func shouldEnqueueDataStoreUsageUpdate(oldTCP, newTCP *kamajiv1alpha1.TenantControlPlane) bool {
	return oldTCP.Status.Storage.DataStoreName != newTCP.Status.Storage.DataStoreName
}

func tenantControlPlaneNamespacedNames(tcpList []kamajiv1alpha1.TenantControlPlane) []string {
	usedBy := make([]string, 0, len(tcpList))
	for _, tcp := range tcpList {
		usedBy = append(usedBy, k8stypes.NamespacedName{Name: tcp.GetName(), Namespace: tcp.GetNamespace()}.String())
	}
	slices.Sort(usedBy)

	return usedBy
}

func equalStringSet(a, b []string) bool {
	a = slices.Clone(a)
	b = slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)

	return slices.Equal(a, b)
}
