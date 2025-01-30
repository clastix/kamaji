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
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type ServiceResource struct {
	resource *corev1.Service
	Client   client.Client
}

func (r *ServiceResource) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	if tenantControlPlane.Spec.Addons.Konnectivity == nil &&
		tenantControlPlane.Status.Addons.Konnectivity.Service.Port == 0 &&
		tenantControlPlane.Status.Addons.Konnectivity.Service.Name == "" &&
		tenantControlPlane.Status.Addons.Konnectivity.Service.Namespace == "" &&
		len(tenantControlPlane.Status.Addons.Konnectivity.Service.ServiceStatus.Conditions) == 0 &&
		len(tenantControlPlane.Status.Addons.Konnectivity.Service.ServiceStatus.LoadBalancer.Ingress) == 0 {
		return false
	}

	if tenantControlPlane.Status.Addons.Konnectivity.Service.Name != r.resource.GetName() ||
		tenantControlPlane.Status.Addons.Konnectivity.Service.Namespace != r.resource.GetNamespace() ||
		len(r.resource.Spec.Ports) > 0 && tenantControlPlane.Status.Addons.Konnectivity.Service.Port != r.resource.Spec.Ports[1].Port ||
		len(r.resource.Spec.Ports) == 0 && tenantControlPlane.Status.Addons.Konnectivity.Service.Port > 0 ||
		len(r.resource.Status.Conditions) != len(tenantControlPlane.Status.Addons.Konnectivity.Service.Conditions) {
		return true
	}

	resourceIngresses, statusIngresses := tenantControlPlane.Status.Addons.Konnectivity.Service.LoadBalancer.Ingress, r.resource.Status.LoadBalancer.Ingress
	if len(resourceIngresses) != len(statusIngresses) {
		return true
	}

	for i := range resourceIngresses {
		if resourceIngresses[i].Hostname != statusIngresses[i].Hostname ||
			resourceIngresses[i].IP != statusIngresses[i].IP ||
			len(resourceIngresses[i].Ports) != len(statusIngresses[i].Ports) {
			return true
		}

		resourcePorts := resourceIngresses[i].Ports
		statusPorts := statusIngresses[i].Ports
		for j := range resourcePorts {
			if resourcePorts[j].Port != statusPorts[j].Port ||
				resourcePorts[j].Protocol != statusPorts[j].Protocol {
				return true
			}
		}
	}

	return false
}

func (r *ServiceResource) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Spec.Addons.Konnectivity == nil && tenantControlPlane.Status.Addons.Konnectivity.Enabled
}

func (r *ServiceResource) CleanUp(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

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
	if err != nil {
		logger.Error(err, "unable to cleanup the resource")

		return false, err
	}

	return res == controllerutil.OperationResultUpdated, nil
}

func (r *ServiceResource) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Addons.Konnectivity.Service = kamajiv1alpha1.KubernetesServiceStatus{}

	if tenantControlPlane.Spec.Addons.Konnectivity != nil {
		tenantControlPlane.Status.Addons.Konnectivity.Service.Name = r.resource.GetName()
		tenantControlPlane.Status.Addons.Konnectivity.Service.Namespace = r.resource.GetNamespace()
		tenantControlPlane.Status.Addons.Konnectivity.Service.Port = r.resource.Spec.Ports[1].Port
		tenantControlPlane.Status.Addons.Konnectivity.Service.ServiceStatus = r.resource.Status
	}

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
	if tenantControlPlane.Spec.Addons.Konnectivity == nil {
		return controllerutil.OperationResultNone, nil
	}

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
		r.resource.Spec.Ports[1].Port = tenantControlPlane.Spec.Addons.Konnectivity.KonnectivityServerSpec.Port
		r.resource.Spec.Ports[1].TargetPort = intstr.FromInt32(tenantControlPlane.Spec.Addons.Konnectivity.KonnectivityServerSpec.Port)
		if tenantControlPlane.Spec.ControlPlane.Service.ServiceType == kamajiv1alpha1.ServiceTypeNodePort {
			r.resource.Spec.Ports[1].NodePort = tenantControlPlane.Spec.Addons.Konnectivity.KonnectivityServerSpec.Port
		}

		return controllerutil.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

func (r *ServiceResource) GetName() string {
	return "konnectivity-service"
}
