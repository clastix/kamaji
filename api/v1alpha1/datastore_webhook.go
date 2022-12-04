// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//+kubebuilder:webhook:path=/mutate-kamaji-clastix-io-v1alpha1-datastore,mutating=true,failurePolicy=fail,sideEffects=None,groups=kamaji.clastix.io,resources=datastores,verbs=create;update,versions=v1alpha1,name=mdatastore.kb.io,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/validate-kamaji-clastix-io-v1alpha1-datastore,mutating=false,failurePolicy=fail,sideEffects=None,groups=kamaji.clastix.io,resources=datastores,verbs=create;update,versions=v1alpha1,name=vdatastore.kb.io,admissionReviewVersions=v1

func (in *DataStore) SetupWebhookWithManager(mgr ctrl.Manager) error {
	validator := &dataStoreValidator{
		log:    mgr.GetLogger().WithName("datastore-webhook"),
		client: mgr.GetClient(),
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(in).
		WithValidator(validator).
		WithDefaulter(validator).
		Complete()
}

type dataStoreValidator struct {
	log    logr.Logger
	client client.Client
}

func (d *dataStoreValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	ds, ok := obj.(*DataStore)
	if !ok {
		return fmt.Errorf("expected *kamajiv1alpha1.DataStore")
	}

	if err := d.validate(ctx, ds); err != nil {
		return err
	}

	return nil
}

func (d *dataStoreValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	old, ok := oldObj.(*DataStore)
	if !ok {
		return fmt.Errorf("expected *kamajiv1alpha1.DataStore")
	}

	ds, ok := newObj.(*DataStore)
	if !ok {
		return fmt.Errorf("expected *kamajiv1alpha1.DataStore")
	}

	d.log.Info("validate update", "name", ds.GetName())

	if ds.Spec.Driver != old.Spec.Driver {
		return fmt.Errorf("driver of a DataStore cannot be changed")
	}

	if err := d.validate(ctx, ds); err != nil {
		return err
	}

	return nil
}

func (d *dataStoreValidator) ValidateDelete(context.Context, runtime.Object) error {
	return nil
}

func (d *dataStoreValidator) Default(context.Context, runtime.Object) error {
	return nil
}

func (d *dataStoreValidator) validate(ctx context.Context, ds *DataStore) error {
	if ds.Spec.BasicAuth != nil {
		if err := d.validateBasicAuth(ctx, ds); err != nil {
			return err
		}
	}

	if err := d.validateTLSConfig(ctx, ds); err != nil {
		return err
	}

	return nil
}

func (d *dataStoreValidator) validateBasicAuth(ctx context.Context, ds *DataStore) error {
	if err := d.validateContentReference(ctx, ds.Spec.BasicAuth.Password); err != nil {
		return fmt.Errorf("basic-auth password is not valid, %w", err)
	}

	if err := d.validateContentReference(ctx, ds.Spec.BasicAuth.Username); err != nil {
		return fmt.Errorf("basic-auth username is not valid, %w", err)
	}

	return nil
}

func (d *dataStoreValidator) validateTLSConfig(ctx context.Context, ds *DataStore) error {
	if err := d.validateContentReference(ctx, ds.Spec.TLSConfig.CertificateAuthority.Certificate); err != nil {
		return fmt.Errorf("CA certificate is not valid, %w", err)
	}

	if ds.Spec.Driver == EtcdDriver {
		if ds.Spec.TLSConfig.CertificateAuthority.PrivateKey == nil {
			return fmt.Errorf("CA private key is required when using the etcd driver")
		}
	}

	if ds.Spec.TLSConfig.CertificateAuthority.PrivateKey != nil {
		if err := d.validateContentReference(ctx, *ds.Spec.TLSConfig.CertificateAuthority.PrivateKey); err != nil {
			return fmt.Errorf("CA private key is not valid, %w", err)
		}
	}

	if err := d.validateContentReference(ctx, ds.Spec.TLSConfig.ClientCertificate.Certificate); err != nil {
		return fmt.Errorf("client certificate is not valid, %w", err)
	}

	if err := d.validateContentReference(ctx, ds.Spec.TLSConfig.ClientCertificate.PrivateKey); err != nil {
		return fmt.Errorf("client private key is not valid, %w", err)
	}

	return nil
}

func (d *dataStoreValidator) validateContentReference(ctx context.Context, ref ContentRef) error {
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

	if err := d.client.Get(ctx, types.NamespacedName{Name: ref.SecretRef.SecretReference.Name, Namespace: ref.SecretRef.SecretReference.Namespace}, &corev1.Secret{}); err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("secret %s/%s is not found", ref.SecretRef.SecretReference.Namespace, ref.SecretRef.SecretReference.Name)
		}

		return err
	}

	return nil
}
