// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type ServiceResource struct {
	resource *corev1.Service
	Client   client.Client
	Name     string
}

func (r *ServiceResource) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	if tenantControlPlane.Status.Addons.Konnectivity.Service.Name != r.resource.GetName() {
		return true
	}

	if tenantControlPlane.Status.Addons.Konnectivity.Service.Namespace != r.resource.GetNamespace() {
		return true
	}

	if tenantControlPlane.Status.Addons.Konnectivity.Service.Port != r.resource.Spec.Ports[1].Port {
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
	res, err := utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, func() error {
		for index, port := range r.resource.Spec.Ports {
			if port.Name == "konnectivity-server" {
				ports := make([]corev1.ServicePort, 0, len(r.resource.Spec.Ports)-1)

				ports = append(ports, r.resource.Spec.Ports[:index]...)
				ports = append(ports, r.resource.Spec.Ports[index+1:]...)

				r.resource.Spec.Ports = ports

				break
			}
		}

		return nil
	})

	return res == controllerutil.OperationResultUpdated, err
}

func (r *ServiceResource) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Spec.Addons.Konnectivity != nil {
		tenantControlPlane.Status.Addons.Konnectivity.Service.Name = r.resource.GetName()
		tenantControlPlane.Status.Addons.Konnectivity.Service.Namespace = r.resource.GetNamespace()
		tenantControlPlane.Status.Addons.Konnectivity.Service.Port = r.resource.Spec.Ports[1].Port
		tenantControlPlane.Status.Addons.Konnectivity.Service.ServiceStatus = r.resource.Status
		tenantControlPlane.Status.Addons.Konnectivity.Enabled = true

		return nil
	}

	tenantControlPlane.Status.Addons.Konnectivity.Service = kamajiv1alpha1.KubernetesServiceStatus{}
	tenantControlPlane.Status.Addons.Konnectivity.Enabled = false

	return nil
}

func (r *ServiceResource) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantControlPlane.GetName(),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *ServiceResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return controllerutil.CreateOrUpdate(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *ServiceResource) mutate(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) func() error {
	return func() error {
		switch len(r.resource.Spec.Ports) {
		case 0:
			return fmt.Errorf("current state of the Service is not ready to be mangled for Konnectivity")
		case 1:
			r.resource.Spec.Ports = append(r.resource.Spec.Ports, corev1.ServicePort{})
		}

		r.resource.Spec.Ports[1].Name = "konnectivity-server"
		r.resource.Spec.Ports[1].Protocol = corev1.ProtocolTCP
		r.resource.Spec.Ports[1].Port = tenantControlPlane.Spec.Addons.Konnectivity.ProxyPort
		r.resource.Spec.Ports[1].TargetPort = intstr.FromInt(int(tenantControlPlane.Spec.Addons.Konnectivity.ProxyPort))

		return controllerutil.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

func (r *ServiceResource) GetName() string {
	return r.Name
}
