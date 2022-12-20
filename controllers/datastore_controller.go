// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

type DataStore struct {
	client client.Client
	// TenantControlPlaneTrigger is the channel used to communicate across the controllers:
	// if a Data Source is updated we have to be sure that the reconciliation of the certificates content
	// for each Tenant Control Plane is put in place properly.
	TenantControlPlaneTrigger TenantControlPlaneChannel
}

//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=datastores,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=datastores/status,verbs=get;update;patch

func (r *DataStore) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)

	ds := &kamajiv1alpha1.DataStore{}
	if err := r.client.Get(ctx, request.NamespacedName, ds); err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		log.Error(err, "unable to retrieve the request")

		return reconcile.Result{}, err
	}

	tcpList := kamajiv1alpha1.TenantControlPlaneList{}

	if err := r.client.List(ctx, &tcpList, client.MatchingFieldsSelector{
		Selector: fields.OneTermEqualSelector(kamajiv1alpha1.TenantControlPlaneUsedDataStoreKey, ds.GetName()),
	}); err != nil {
		log.Error(err, "cannot retrieve list of the Tenant Control Plane using the following instance")

		return reconcile.Result{}, err
	}
	// Updating the status with the list of Tenant Control Plane using the following Data Source
	tcpSets := sets.NewString()
	for _, tcp := range tcpList.Items {
		tcpSets.Insert(getNamespacedName(tcp.GetNamespace(), tcp.GetName()).String())
	}

	ds.Status.UsedBy = tcpSets.List()

	if err := r.client.Status().Update(ctx, ds); err != nil {
		log.Error(err, "cannot update the status for the given instance")

		return reconcile.Result{}, err
	}
	// Triggering the reconciliation of the Tenant Control Plane upon a Secret change
	for _, i := range tcpList.Items {
		tcp := i

		r.TenantControlPlaneTrigger <- event.GenericEvent{Object: &tcp}
	}

	return reconcile.Result{}, nil
}

func (r *DataStore) InjectClient(client client.Client) error {
	r.client = client

	return nil
}

func (r *DataStore) SetupWithManager(mgr controllerruntime.Manager) error {
	enqueueFn := func(tcp *kamajiv1alpha1.TenantControlPlane, limitingInterface workqueue.RateLimitingInterface) {
		if dataStoreName := tcp.Status.Storage.DataStoreName; len(dataStoreName) > 0 {
			limitingInterface.AddRateLimited(reconcile.Request{
				NamespacedName: k8stypes.NamespacedName{
					Name: dataStoreName,
				},
			})
		}
	}
	//nolint:forcetypeassert
	return controllerruntime.NewControllerManagedBy(mgr).
		For(&kamajiv1alpha1.DataStore{}, builder.WithPredicates(
			predicate.ResourceVersionChangedPredicate{},
		)).
		Watches(&source.Kind{Type: &kamajiv1alpha1.TenantControlPlane{}}, handler.Funcs{
			CreateFunc: func(createEvent event.CreateEvent, limitingInterface workqueue.RateLimitingInterface) {
				enqueueFn(createEvent.Object.(*kamajiv1alpha1.TenantControlPlane), limitingInterface)
			},
			UpdateFunc: func(updateEvent event.UpdateEvent, limitingInterface workqueue.RateLimitingInterface) {
				enqueueFn(updateEvent.ObjectOld.(*kamajiv1alpha1.TenantControlPlane), limitingInterface)
				enqueueFn(updateEvent.ObjectNew.(*kamajiv1alpha1.TenantControlPlane), limitingInterface)
			},
			DeleteFunc: func(deleteEvent event.DeleteEvent, limitingInterface workqueue.RateLimitingInterface) {
				enqueueFn(deleteEvent.Object.(*kamajiv1alpha1.TenantControlPlane), limitingInterface)
			},
		}).
		Complete(r)
}
