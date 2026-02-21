// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/controllers/utils"
)

type DataStore struct {
	Client client.Client
	// TenantControlPlaneTrigger is the channel used to communicate across the controllers:
	// if a Data Source is updated, we have to be sure that the reconciliation of the certificates content
	// for each Tenant Control Plane is put in place properly.
	TenantControlPlaneTrigger chan event.GenericEvent
}

//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=datastores,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=datastores/status,verbs=get;update;patch

func (r *DataStore) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	var err error

	logger := log.FromContext(ctx)

	var ds kamajiv1alpha1.DataStore
	if dsErr := r.Client.Get(ctx, request.NamespacedName, &ds); dsErr != nil {
		if k8serrors.IsNotFound(dsErr) {
			logger.Info("resource may have been deleted, skipping")

			return reconcile.Result{}, nil
		}

		logger.Error(dsErr, "cannot retrieve the required resource")

		return reconcile.Result{}, dsErr
	}

	if utils.IsPaused(&ds) {
		logger.Info("paused reconciliation, no further actions")

		return reconcile.Result{}, nil
	}

	if ds.GetDeletionTimestamp() == nil && !controllerutil.ContainsFinalizer(&ds, kamajiv1alpha1.DataStoreTCPFinalizer) {
		logger.Info("missing finalizer, adding it")

		ds.SetFinalizers(append(ds.GetFinalizers(), kamajiv1alpha1.DataStoreTCPFinalizer))
		if uErr := r.Client.Update(ctx, &ds); uErr != nil {
			return reconcile.Result{}, uErr
		}

		return reconcile.Result{}, nil
	}
	// Extracting the list of TenantControlPlane objects referenced by the given DataStore:
	// this data is used to reference these in the Status, as well as propagating changes
	// that would be required, such as changing TLS Configuration, or Basic Auth.
	var tcpList kamajiv1alpha1.TenantControlPlaneList

	if lErr := r.Client.List(ctx, &tcpList, client.MatchingFieldsSelector{
		Selector: fields.OneTermEqualSelector(kamajiv1alpha1.TenantControlPlaneUsedDataStoreKey, ds.GetName()),
	}); lErr != nil {
		return reconcile.Result{}, fmt.Errorf("cannot retrieve list of the Tenant Control Plane using the following instance: %w", lErr)
	}

	tcpSets := sets.NewString()

	for _, tcp := range tcpList.Items {
		tcpSets.Insert(getNamespacedName(tcp.GetNamespace(), tcp.GetName()).String())
	}

	ds.Status.UsedBy = tcpSets.List()
	// Performing the status update only at the end of the reconciliation loop:
	// this is performed in defer to avoid duplication of code,
	// and triggering a reconciliation of depending on TenantControlPlanes only if the update was successful.
	defer func() {
		if meta.IsStatusConditionTrue(ds.Status.Conditions, kamajiv1alpha1.DataStoreConditionAllowedDeletionType) {
			logger.Info("removing finalizer upon true condition")

			controllerutil.RemoveFinalizer(&ds, kamajiv1alpha1.DataStoreTCPFinalizer)
			if uErr := r.Client.Update(ctx, &ds); uErr != nil {
				logger.Error(uErr, "cannot update object")

				return
			}

			logger.Info("finalizer removed successfully")

			return
		}

		ds.Status.ObservedGeneration = ds.Generation
		ds.Status.Ready = meta.IsStatusConditionTrue(ds.Status.Conditions, kamajiv1alpha1.DataStoreConditionValidType)

		if err = r.Client.Status().Update(ctx, &ds); err != nil {
			logger.Error(err, "cannot update the status for the given instance")

			return
		}

		if !ds.Status.Ready {
			logger.Info("skipping triggering, DataStore is not ready")

			return
		}

		logger.Info("triggering cascading reconciliation for TenantControlPlanes")

		for _, tcp := range tcpList.Items {
			var shrunkTCP kamajiv1alpha1.TenantControlPlane

			shrunkTCP.Name = tcp.Name
			shrunkTCP.Namespace = tcp.Namespace

			go utils.TriggerChannel(ctx, r.TenantControlPlaneTrigger, shrunkTCP)
		}
	}()

	if ds.GetDeletionTimestamp() != nil {
		if len(tcpList.Items) > 0 {
			logger.Info("deletion is blocked due to DataStore still being referenced")

			meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
				Type:               kamajiv1alpha1.DataStoreConditionAllowedDeletionType,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: ds.Generation,
				Reason:             "DataStoreStillUsed",
				Message:            "The DataStore is still used and referenced by one (or more) TenantControlPlane objects.",
			})

			return reconcile.Result{}, nil
		}

		if meta.IsStatusConditionFalse(ds.Status.Conditions, kamajiv1alpha1.DataStoreConditionAllowedDeletionType) {
			logger.Info("DataStore is not used by any TenantControlPlane object")

			meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
				Type:               kamajiv1alpha1.DataStoreConditionAllowedDeletionType,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: ds.Generation,
				Reason:             "DataStoreUnused",
				Message:            "",
			})

			return reconcile.Result{}, nil
		}

		logger.Info("DataStore can be safely deleted")

		return reconcile.Result{}, nil
	}

	if exists := meta.FindStatusCondition(ds.Status.Conditions, kamajiv1alpha1.DataStoreConditionValidType); exists == nil {
		logger.Info("missing starting condition")

		meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
			Type:               kamajiv1alpha1.DataStoreConditionValidType,
			Status:             metav1.ConditionUnknown,
			ObservedGeneration: ds.Generation,
			Reason:             "MissingCondition",
			Message:            "Controller will process the validation.",
		})

		if sErr := r.Client.Status().Update(ctx, &ds); sErr != nil {
			return reconcile.Result{}, fmt.Errorf("cannot update the status for the given instance: %w", sErr)
		}

		return reconcile.Result{}, nil
	}

	if ds.Spec.BasicAuth != nil {
		logger.Info("validating basic authentication")

		if vErr := r.validateBasicAuth(ctx, ds); vErr != nil {
			meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
				Type:               kamajiv1alpha1.DataStoreConditionValidType,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: ds.Generation,
				Reason:             "BasicAuthValidationFailed",
				Message:            vErr.Error(),
			})

			logger.Info("invalid basic authentication")

			return reconcile.Result{}, nil
		}

		logger.Info("basic authentication is valid")
	}

	logger.Info("validating TLS configuration")

	if vErr := r.validateTLSConfig(ctx, ds); vErr != nil {
		meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
			Type:               kamajiv1alpha1.DataStoreConditionValidType,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: ds.Generation,
			Reason:             "TLSConfigurationValidationFailed",
			Message:            vErr.Error(),
		})

		logger.Info("invalid TLS configuration")

		return reconcile.Result{}, nil
	}

	logger.Info("TLS configuration is valid")

	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:               kamajiv1alpha1.DataStoreConditionValidType,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: ds.Status.ObservedGeneration,
		Reason:             "DataStoreIsValid",
		Message:            "",
	})

	return reconcile.Result{}, err
}

func (r *DataStore) SetupWithManager(mgr controllerruntime.Manager) error {
	enqueueFn := func(tcp *kamajiv1alpha1.TenantControlPlane, limitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
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
		For(&kamajiv1alpha1.DataStore{}).
		Watches(&kamajiv1alpha1.TenantControlPlane{}, handler.Funcs{
			CreateFunc: func(_ context.Context, createEvent event.TypedCreateEvent[client.Object], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				enqueueFn(createEvent.Object.(*kamajiv1alpha1.TenantControlPlane), w)
			},
			UpdateFunc: func(_ context.Context, updateEvent event.TypedUpdateEvent[client.Object], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				enqueueFn(updateEvent.ObjectOld.(*kamajiv1alpha1.TenantControlPlane), w)
				enqueueFn(updateEvent.ObjectNew.(*kamajiv1alpha1.TenantControlPlane), w)
			},
			DeleteFunc: func(_ context.Context, deleteEvent event.TypedDeleteEvent[client.Object], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				enqueueFn(deleteEvent.Object.(*kamajiv1alpha1.TenantControlPlane), w)
			},
		}).
		Complete(r)
}

func (r *DataStore) validateBasicAuth(ctx context.Context, ds kamajiv1alpha1.DataStore) error {
	if err := r.validateContentReference(ctx, ds.Spec.BasicAuth.Password); err != nil {
		return fmt.Errorf("basic-auth password is not valid, %w", err)
	}

	if err := r.validateContentReference(ctx, ds.Spec.BasicAuth.Username); err != nil {
		return fmt.Errorf("basic-auth username is not valid, %w", err)
	}

	return nil
}

func (r *DataStore) validateTLSConfig(ctx context.Context, ds kamajiv1alpha1.DataStore) error {
	if ds.Spec.TLSConfig == nil && ds.Spec.Driver != kamajiv1alpha1.EtcdDriver {
		return nil
	}

	if err := r.validateContentReference(ctx, ds.Spec.TLSConfig.CertificateAuthority.Certificate); err != nil {
		return fmt.Errorf("CA certificate is not valid, %w", err)
	}

	if ds.Spec.Driver == kamajiv1alpha1.EtcdDriver {
		if ds.Spec.TLSConfig.CertificateAuthority.PrivateKey == nil {
			return fmt.Errorf("CA private key is required when using the etcd driver")
		}

		if ds.Spec.TLSConfig.ClientCertificate == nil {
			return fmt.Errorf("client certificate is required when using the etcd driver")
		}
	}

	if ds.Spec.TLSConfig.CertificateAuthority.PrivateKey != nil {
		if err := r.validateContentReference(ctx, *ds.Spec.TLSConfig.CertificateAuthority.PrivateKey); err != nil {
			return fmt.Errorf("CA private key is not valid, %w", err)
		}
	}

	if ds.Spec.TLSConfig.ClientCertificate != nil {
		if err := r.validateContentReference(ctx, ds.Spec.TLSConfig.ClientCertificate.Certificate); err != nil {
			return fmt.Errorf("client certificate is not valid, %w", err)
		}

		if err := r.validateContentReference(ctx, ds.Spec.TLSConfig.ClientCertificate.PrivateKey); err != nil {
			return fmt.Errorf("client private key is not valid, %w", err)
		}
	}

	return nil
}

func (r *DataStore) validateContentReference(ctx context.Context, ref kamajiv1alpha1.ContentRef) error {
	switch {
	case len(ref.Content) > 0:
		return nil
	case ref.SecretRef == nil:
		return fmt.Errorf("the Secret reference is mandatory when bare content is not specified")
	case len(ref.SecretRef.SecretReference.Name) == 0:
		return fmt.Errorf("the Secret reference name is mandatory")
	case len(ref.SecretRef.SecretReference.Namespace) == 0:
		return fmt.Errorf("the Secret reference namespace is mandatory")
	}

	if err := r.Client.Get(ctx, k8stypes.NamespacedName{Name: ref.SecretRef.SecretReference.Name, Namespace: ref.SecretRef.SecretReference.Namespace}, &corev1.Secret{}); err != nil {
		if k8serrors.IsNotFound(err) {
			return fmt.Errorf("secret %s/%s is not found", ref.SecretRef.SecretReference.Namespace, ref.SecretRef.SecretReference.Name)
		}

		return err
	}

	return nil
}
