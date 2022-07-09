// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type KubernetesIngressResource struct {
	resource *networkingv1.Ingress
	Client   client.Client
	Name     string
}

func (r *KubernetesIngressResource) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return !(tenantControlPlane.Status.Kubernetes.Ingress.Name == r.resource.GetName() &&
		tenantControlPlane.Status.Kubernetes.Ingress.Namespace == r.resource.GetNamespace())
}

func (r *KubernetesIngressResource) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return !tenantControlPlane.Spec.ControlPlane.Ingress.Enabled
}

func (r *KubernetesIngressResource) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	if err := r.Client.Delete(ctx, r.resource); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (r *KubernetesIngressResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Spec.ControlPlane.Ingress.Enabled {
		tenantControlPlane.Status.Kubernetes.Ingress.IngressStatus = r.resource.Status
		tenantControlPlane.Status.Kubernetes.Ingress.Name = r.resource.GetName()
		tenantControlPlane.Status.Kubernetes.Ingress.Namespace = r.resource.GetNamespace()

		return nil
	}

	tenantControlPlane.Status.Kubernetes.Ingress = kamajiv1alpha1.KubernetesIngressStatus{}

	return nil
}

func (r *KubernetesIngressResource) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantControlPlane.GetName(),
			Namespace: tenantControlPlane.GetNamespace(),
			Labels:    utilities.CommonLabels(tenantControlPlane.GetName()),
		},
	}

	r.Name = "ingress"

	return nil
}

func (r *KubernetesIngressResource) mutate(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		labels := utilities.MergeMaps(r.resource.GetLabels(), tenantControlPlane.Spec.ControlPlane.Ingress.AdditionalMetadata.Labels)
		r.resource.SetLabels(labels)

		annotations := utilities.MergeMaps(r.resource.GetAnnotations(), tenantControlPlane.Spec.ControlPlane.Ingress.AdditionalMetadata.Annotations)
		r.resource.SetAnnotations(annotations)

		if tenantControlPlane.Spec.ControlPlane.Ingress.IngressClassName != "" {
			r.resource.Spec.IngressClassName = &tenantControlPlane.Spec.ControlPlane.Ingress.IngressClassName
		}

		var rule networkingv1.IngressRule
		if len(r.resource.Spec.Rules) > 0 {
			rule = r.resource.Spec.Rules[0]
		}

		var path networkingv1.HTTPIngressPath
		if rule.HTTP != nil && len(rule.HTTP.Paths) > 0 {
			path = rule.HTTP.Paths[0]
		}

		path.Path = "/"
		path.PathType = (*networkingv1.PathType)(pointer.StringPtr(string(networkingv1.PathTypePrefix)))

		if path.Backend.Service == nil {
			path.Backend.Service = &networkingv1.IngressServiceBackend{}
		}

		if tenantControlPlane.Status.Kubernetes.Service.Name == "" ||
			tenantControlPlane.Status.Kubernetes.Service.Port == 0 {
			return fmt.Errorf("ingress cannot be configured yet")
		}

		path.Backend.Service.Name = tenantControlPlane.Status.Kubernetes.Service.Name
		path.Backend.Service.Port.Number = tenantControlPlane.Status.Kubernetes.Service.Port

		if rule.HTTP == nil {
			rule.HTTP = &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{
					{},
				},
			}
		}

		rule.HTTP.Paths[0] = path

		if len(tenantControlPlane.Spec.ControlPlane.Ingress.Hostname) == 0 {
			return fmt.Errorf("missing hostname to expose the Tenant Control Plane using an Ingress resource")
		}

		rule.Host = tenantControlPlane.Spec.ControlPlane.Ingress.Hostname

		r.resource.Spec.Rules = []networkingv1.IngressRule{
			rule,
		}

		return controllerutil.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

func (r *KubernetesIngressResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(tenantControlPlane))
}

func (r *KubernetesIngressResource) GetName() string {
	return r.Name
}
