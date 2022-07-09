// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

// KubernetesServiceResource must be the first Resource processed by the TenantControlPlane:
// when a TenantControlPlan is expecting a dynamic IP address, the Service will get it from the controller-manager.
type KubernetesServiceResource struct {
	resource *corev1.Service
	Client   client.Client
	Name     string
}

func (r *KubernetesServiceResource) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Kubernetes.Service.Name != r.resource.GetName() ||
		tenantControlPlane.Status.Kubernetes.Service.Namespace != r.resource.GetNamespace() ||
		tenantControlPlane.Status.Kubernetes.Service.Port != r.resource.Spec.Ports[0].Port
}

func (r *KubernetesServiceResource) ShouldCleanup(plane *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *KubernetesServiceResource) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *KubernetesServiceResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Kubernetes.Service.ServiceStatus = r.resource.Status
	tenantControlPlane.Status.Kubernetes.Service.Name = r.resource.GetName()
	tenantControlPlane.Status.Kubernetes.Service.Namespace = r.resource.GetNamespace()
	tenantControlPlane.Status.Kubernetes.Service.Port = r.resource.Spec.Ports[0].Port

	return nil
}

func (r *KubernetesServiceResource) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantControlPlane.GetName(),
			Namespace: tenantControlPlane.GetNamespace(),
			Labels:    utilities.CommonLabels(tenantControlPlane.GetName()),
		},
	}

	r.Name = "service"

	return nil
}

func (r *KubernetesServiceResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *KubernetesServiceResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	// We don't need to check error here: in case of dynamic external IP, the Service must be created in advance.
	// After that, the specific cloud controller-manager will provide an IP that will be then used.
	address, _ := tenantControlPlane.GetControlPlaneAddress(ctx, r.Client)

	return func() error {
		var servicePort corev1.ServicePort
		if len(r.resource.Spec.Ports) > 0 {
			servicePort = r.resource.Spec.Ports[0]
		}
		servicePort.Protocol = corev1.ProtocolTCP
		servicePort.Port = tenantControlPlane.Spec.NetworkProfile.Port
		servicePort.TargetPort = intstr.FromInt(int(tenantControlPlane.Spec.NetworkProfile.Port))

		r.resource.Spec.Ports = []corev1.ServicePort{servicePort}
		r.resource.Spec.Selector = map[string]string{
			"kamaji.clastix.io/soot": tenantControlPlane.GetName(),
		}

		labels := utilities.MergeMaps(r.resource.GetLabels(), tenantControlPlane.Spec.ControlPlane.Service.AdditionalMetadata.Labels)
		r.resource.SetLabels(labels)

		annotations := utilities.MergeMaps(r.resource.GetAnnotations(), tenantControlPlane.Spec.ControlPlane.Service.AdditionalMetadata.Annotations)
		r.resource.SetAnnotations(annotations)

		switch tenantControlPlane.Spec.ControlPlane.Service.ServiceType {
		case kamajiv1alpha1.ServiceTypeLoadBalancer:
			r.resource.Spec.Type = corev1.ServiceTypeLoadBalancer

			if len(address) > 0 {
				r.resource.Spec.LoadBalancerIP = address
			}
		case kamajiv1alpha1.ServiceTypeNodePort:
			r.resource.Spec.Type = corev1.ServiceTypeNodePort
			r.resource.Spec.Ports[0].NodePort = tenantControlPlane.Spec.NetworkProfile.Port

			if tenantControlPlane.Spec.NetworkProfile.AllowAddressAsExternalIP && len(address) > 0 {
				r.resource.Spec.ExternalIPs = []string{address}
			}
		default:
			r.resource.Spec.Type = corev1.ServiceTypeClusterIP

			if tenantControlPlane.Spec.NetworkProfile.AllowAddressAsExternalIP && len(address) > 0 {
				r.resource.Spec.ExternalIPs = []string{address}
			}
		}

		return controllerutil.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

func (r *KubernetesServiceResource) GetName() string {
	return r.Name
}
