// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pointer "k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type KubernetesIngressResource struct {
	resource *networkingv1.Ingress
	Client   client.Client
}

func (r *KubernetesIngressResource) GetHistogram() prometheus.Histogram {
	ingressCollector = LazyLoadHistogramFromResource(ingressCollector, r)

	return ingressCollector
}

func (r *KubernetesIngressResource) ShouldStatusBeUpdated(_ context.Context, tcp *kamajiv1alpha1.TenantControlPlane) bool {
	switch {
	case tcp.Spec.ControlPlane.Ingress == nil && tcp.Status.Kubernetes.Ingress == nil:
		// No update in case of no ingress in spec, neither in status.
		return false
	case tcp.Spec.ControlPlane.Ingress != nil && tcp.Status.Kubernetes.Ingress == nil, // TCP is using an Ingress, Status not tracking it
		tcp.Spec.ControlPlane.Ingress == nil && tcp.Status.Kubernetes.Ingress != nil: // Status tracks an Ingress, Spec doesn't
		return true
	case len(tcp.Status.Kubernetes.Ingress.IngressStatus.LoadBalancer.Ingress) != len(r.resource.Status.LoadBalancer.Ingress):
		// Mismatch count of tracked LoadBalancer Ingress
		return true
	default:
		statusIngress := tcp.Status.Kubernetes.Ingress.IngressStatus.LoadBalancer.Ingress

		for i, ingress := range r.resource.Status.LoadBalancer.Ingress {
			if ingress.IP != statusIngress[i].IP {
				return true
			}

			if len(ingress.Ports) != len(statusIngress[i].Ports) {
				return true
			}

			for p, port := range ingress.Ports {
				if port.Port != statusIngress[i].Ports[p].Port {
					return true
				}

				if port.Protocol != statusIngress[i].Ports[p].Protocol {
					return true
				}

				if port.Error == nil && statusIngress[i].Ports[p].Error != nil ||
					port.Error != nil && statusIngress[i].Ports[p].Error == nil {
					return true
				}

				if port.Error == nil && statusIngress[i].Ports[p].Error == nil {
					continue
				}

				if *port.Error != *statusIngress[i].Ports[p].Error {
					return true
				}
			}
		}

		return false
	}
}

func (r *KubernetesIngressResource) ShouldCleanup(tcp *kamajiv1alpha1.TenantControlPlane) bool {
	return tcp.Spec.ControlPlane.Ingress == nil
}

func (r *KubernetesIngressResource) CleanUp(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	var ingress networkingv1.Ingress
	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: r.resource.GetNamespace(),
		Name:      r.resource.GetName(),
	}, &ingress); err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error(err, "failed to get ingress resource before cleanup")

			return false, err
		}

		return false, nil
	}

	if !metav1.IsControlledBy(&ingress, tcp) {
		logger.Info("skipping cleanup: ingress is not managed by Kamaji", "name", ingress.Name, "namespace", ingress.Namespace)

		return false, nil
	}

	if err := r.Client.Delete(ctx, &ingress); err != nil {
		if !k8serrors.IsNotFound(err) {
			logger.Error(err, "cannot cleanup resource")

			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (r *KubernetesIngressResource) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Spec.ControlPlane.Ingress != nil {
		tenantControlPlane.Status.Kubernetes.Ingress = &kamajiv1alpha1.KubernetesIngressStatus{
			IngressStatus: r.resource.Status,
			Name:          r.resource.GetName(),
			Namespace:     r.resource.GetNamespace(),
		}

		return nil
	}

	tenantControlPlane.Status.Kubernetes.Ingress = nil

	return nil
}

func (r *KubernetesIngressResource) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantControlPlane.GetName(),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *KubernetesIngressResource) mutate(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		labels := utilities.MergeMaps(utilities.KamajiLabels(tenantControlPlane.GetName(), r.GetName()), tenantControlPlane.Spec.ControlPlane.Ingress.AdditionalMetadata.Labels)
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
		path.PathType = (*networkingv1.PathType)(pointer.To(string(networkingv1.PathTypePrefix)))

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

		rule.Host, _ = utilities.GetControlPlaneAddressAndPortFromHostname(tenantControlPlane.Spec.ControlPlane.Ingress.Hostname, 0)

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
	return "ingress"
}
