// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"context"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

// ServiceResource must be the first Resource processed by the TenantControlPlane:
// when a TenantControlPlan is expecting a dynamic IP address, the Service will get it from the controller-manager.
type ServiceResource struct {
	resource *corev1.Service
	Client   client.Client
	Name     string
}

func (r *ServiceResource) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	if tenantControlPlane.Status.Addons.Konnectivity.Service.Name != r.resource.GetName() {
		return true
	}

	if tenantControlPlane.Status.Addons.Konnectivity.Service.Namespace != r.resource.GetNamespace() {
		return true
	}

	if tenantControlPlane.Status.Addons.Konnectivity.Service.Port != r.resource.Spec.Ports[0].Port {
		return true
	}

	if len(r.resource.Status.Conditions) != len(tenantControlPlane.Status.Addons.Konnectivity.Service.Conditions) {
		return true
	}

	resourceIngresses := tenantControlPlane.Status.Addons.Konnectivity.Service.LoadBalancer.Ingress
	statusIngresses := r.resource.Status.LoadBalancer.Ingress

	if len(resourceIngresses) != len(statusIngresses) {
		return true
	}

	for i := 0; i < len(resourceIngresses); i++ {
		if resourceIngresses[i].Hostname != statusIngresses[i].Hostname ||
			resourceIngresses[i].IP != statusIngresses[i].IP ||
			len(resourceIngresses[i].Ports) != len(statusIngresses[i].Ports) {
			return true
		}

		resourcePorts := resourceIngresses[i].Ports
		statusPorts := statusIngresses[i].Ports
		for j := 0; j < len(resourcePorts); j++ {
			if resourcePorts[j].Port != statusPorts[j].Port ||
				resourcePorts[j].Protocol != statusPorts[j].Protocol {
				return true
			}
		}
	}

	return false
}

func (r *ServiceResource) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Spec.Addons.Konnectivity == nil
}

func (r *ServiceResource) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	if err := r.Client.Delete(ctx, r.resource); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (r *ServiceResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Spec.Addons.Konnectivity != nil {
		tenantControlPlane.Status.Addons.Konnectivity.Service.Name = r.resource.GetName()
		tenantControlPlane.Status.Addons.Konnectivity.Service.Namespace = r.resource.GetNamespace()
		tenantControlPlane.Status.Addons.Konnectivity.Service.Port = r.resource.Spec.Ports[0].Port
		tenantControlPlane.Status.Addons.Konnectivity.Service.ServiceStatus = r.resource.Status
		tenantControlPlane.Status.Addons.Konnectivity.Enabled = true

		return nil
	}

	tenantControlPlane.Status.Addons.Konnectivity.Service = kamajiv1alpha1.KubernetesServiceStatus{}
	tenantControlPlane.Status.Addons.Konnectivity.Enabled = false

	return nil
}

func (r *ServiceResource) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *ServiceResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return controllerutil.CreateOrUpdate(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *ServiceResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) func() error {
	return func() (err error) {
		address, _ := tenantControlPlane.GetKonnectivityServerAddress(ctx, r.Client)
		if address == "" {
			// The TenantControlPlane is getting a dynamic IP address: retrieving from the status
			address, _, err = net.SplitHostPort(tenantControlPlane.Status.ControlPlaneEndpoint)
			if err != nil {
				return err
			}
		}

		var servicePort corev1.ServicePort
		if len(r.resource.Spec.Ports) > 0 {
			servicePort = r.resource.Spec.Ports[0]
		}
		servicePort.Protocol = corev1.ProtocolTCP
		servicePort.Port = tenantControlPlane.Spec.Addons.Konnectivity.ProxyPort
		servicePort.TargetPort = intstr.FromInt(int(tenantControlPlane.Spec.Addons.Konnectivity.ProxyPort))

		r.resource.Spec.Ports = []corev1.ServicePort{servicePort}
		r.resource.Spec.Selector = map[string]string{
			"kamaji.clastix.io/soot": tenantControlPlane.GetName(),
		}

		labels := utilities.MergeMaps(r.resource.GetLabels(), tenantControlPlane.Spec.ControlPlane.Service.AdditionalMetadata.Labels)
		r.resource.SetLabels(labels)

		annotations := utilities.MergeMaps(r.resource.GetAnnotations(), tenantControlPlane.Spec.ControlPlane.Service.AdditionalMetadata.Annotations)
		r.resource.SetAnnotations(annotations)

		isIP := false

		switch {
		case utilities.IsValidIP(address):
			isIP = true
		case !utilities.IsValidHostname(address):
			return fmt.Errorf("%s is not a valid address for konnectivity proxy server", address)
		}

		switch tenantControlPlane.Spec.Addons.Konnectivity.ServiceType {
		case kamajiv1alpha1.ServiceTypeLoadBalancer:
			r.resource.Spec.Type = corev1.ServiceTypeLoadBalancer

			if isIP {
				r.resource.Spec.LoadBalancerIP = address
			}
		case kamajiv1alpha1.ServiceTypeNodePort:
			r.resource.Spec.Type = corev1.ServiceTypeNodePort
			r.resource.Spec.Ports[0].NodePort = tenantControlPlane.Spec.Addons.Konnectivity.ProxyPort

			if isIP && tenantControlPlane.Spec.Addons.Konnectivity.AllowAddressAsExternalIP {
				r.resource.Spec.ExternalIPs = []string{address}
			}
		default:
			r.resource.Spec.Type = corev1.ServiceTypeClusterIP

			if isIP && tenantControlPlane.Spec.Addons.Konnectivity.AllowAddressAsExternalIP {
				r.resource.Spec.ExternalIPs = []string{address}
			}
		}

		return controllerutil.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

func (r *ServiceResource) GetName() string {
	return r.Name
}

func (r *ServiceResource) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.Name, tenantControlPlane)
}
