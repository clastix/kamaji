// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"net"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
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
}

func (r *KubernetesServiceResource) GetHistogram() prometheus.Histogram {
	serviceCollector = LazyLoadHistogramFromResource(serviceCollector, r)

	return serviceCollector
}

func (r *KubernetesServiceResource) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Kubernetes.Service.Name != r.resource.GetName() ||
		tenantControlPlane.Status.Kubernetes.Service.Namespace != r.resource.GetNamespace() ||
		tenantControlPlane.Status.Kubernetes.Service.Port != r.resource.Spec.Ports[0].Port
}

func (r *KubernetesServiceResource) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *KubernetesServiceResource) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *KubernetesServiceResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Kubernetes.Service.ServiceStatus = r.resource.Status
	tenantControlPlane.Status.Kubernetes.Service.Name = r.resource.GetName()
	tenantControlPlane.Status.Kubernetes.Service.Namespace = r.resource.GetNamespace()
	tenantControlPlane.Status.Kubernetes.Service.Port = r.resource.Spec.Ports[0].Port

	address, err := tenantControlPlane.DeclaredControlPlaneAddress(ctx, r.Client)
	if err != nil {
		return err
	}
	tenantControlPlane.Status.ControlPlaneEndpoint = net.JoinHostPort(address, strconv.FormatInt(int64(tenantControlPlane.Spec.NetworkProfile.Port), 10))

	return nil
}

func (r *KubernetesServiceResource) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantControlPlane.GetName(),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *KubernetesServiceResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *KubernetesServiceResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	// We don't need to check error here: in case of dynamic external IP, the Service must be created in advance.
	// After that, the specific cloud controller-manager will provide an IP that will be then used.
	address, _ := tenantControlPlane.DeclaredControlPlaneAddress(ctx, r.Client)

	return func() error {
		labels := utilities.MergeMaps(
			r.resource.GetLabels(),
			utilities.KamajiLabels(
				tenantControlPlane.GetName(), r.GetName()),
			tenantControlPlane.Spec.ControlPlane.Service.AdditionalMetadata.Labels,
		)
		r.resource.SetLabels(labels)

		annotations := utilities.MergeMaps(r.resource.GetAnnotations(), tenantControlPlane.Spec.ControlPlane.Service.AdditionalMetadata.Annotations)
		r.resource.SetAnnotations(annotations)

		r.resource.Spec.Selector = map[string]string{
			"kamaji.clastix.io/name": tenantControlPlane.GetName(),
		}

		if r.resource.Spec.Ports == nil {
			r.resource.Spec.Ports = make([]corev1.ServicePort, 1)
		}

		var ports []corev1.ServicePort
		for i, port := range r.resource.Spec.Ports {
			switch {
			case i == 0:
				port.Name = "kube-apiserver"
				port.Protocol = corev1.ProtocolTCP
				port.Port = tenantControlPlane.Spec.NetworkProfile.Port
				port.TargetPort = intstr.FromInt32(tenantControlPlane.Spec.NetworkProfile.Port)

				ports = append(ports, port)
			case i == 1 && port.Name == "konnectivity-server":
				ports = append(ports, port)
			}
		}

		for _, port := range tenantControlPlane.Spec.ControlPlane.Service.AdditionalPorts {
			ports = append(ports, corev1.ServicePort{
				Name:        port.Name,
				Protocol:    port.Protocol,
				AppProtocol: port.AppProtocol,
				Port:        port.Port,
				TargetPort:  port.TargetPort,
				NodePort:    0,
			})
		}

		r.resource.Spec.Ports = ports

		switch tenantControlPlane.Spec.ControlPlane.Service.ServiceType {
		case kamajiv1alpha1.ServiceTypeLoadBalancer:
			r.resource.Spec.Type = corev1.ServiceTypeLoadBalancer

			if tenantControlPlane.Spec.NetworkProfile.LoadBalancerClass != nil {
				r.resource.Spec.LoadBalancerClass = ptr.To(*tenantControlPlane.Spec.NetworkProfile.LoadBalancerClass)
			}

			if len(tenantControlPlane.Spec.NetworkProfile.LoadBalancerSourceRanges) > 0 {
				r.resource.Spec.LoadBalancerSourceRanges = tenantControlPlane.Spec.NetworkProfile.LoadBalancerSourceRanges
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
	return "service"
}
