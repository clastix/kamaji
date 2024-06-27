// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"

	"github.com/clastix/kamaji-telemetry/api"
	kamajitelemetry "github.com/clastix/kamaji-telemetry/pkg/client"
	"k8s.io/apimachinery/pkg/runtime"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/webhook/utils"
)

type TenantControlPlaneTelemetry struct {
	Enabled           bool
	TelemetryClient   kamajitelemetry.Client
	KamajiVersion     string
	KubernetesVersion string
}

func (t TenantControlPlaneTelemetry) OnCreate(object runtime.Object) AdmissionResponse {
	if t.Enabled {
		tcp := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		go t.TelemetryClient.PushCreate(context.Background(), api.Create{
			KamajiVersion:     t.KamajiVersion,
			KubernetesVersion: t.KubernetesVersion,
			TenantVersion:     tcp.Spec.Kubernetes.Version,
			ClusterAPIOwned: func() bool {
				for _, owner := range tcp.OwnerReferences {
					if owner.Kind == "KamajiControlPlane" {
						return true
					}
				}

				return false
			}(),
			NetworkProfile: func() string {
				switch {
				case tcp.Spec.ControlPlane.Ingress != nil:
					return api.NetworkProfileIngress
				case tcp.Spec.ControlPlane.Service.ServiceType == kamajiv1alpha1.ServiceTypeLoadBalancer:
					return api.NetworkProfileLB
				case tcp.Spec.ControlPlane.Service.ServiceType == kamajiv1alpha1.ServiceTypeNodePort:
					return api.NetworkProfileNodePort
				case tcp.Spec.ControlPlane.Service.ServiceType == kamajiv1alpha1.ServiceTypeClusterIP:
					return api.NetworkProfileClusterIP
				default:
					return "Unknown"
				}
			}(),
		})
	}

	return utils.NilOp()
}

func (t TenantControlPlaneTelemetry) OnDelete(object runtime.Object) AdmissionResponse {
	if t.Enabled {
		tcp := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		go t.TelemetryClient.PushDelete(context.Background(), api.Delete{
			KamajiVersion:     t.KamajiVersion,
			KubernetesVersion: t.KubernetesVersion,
			TenantVersion:     tcp.Spec.Kubernetes.Version,
			Status:            t.extractTCPVersion(tcp.Status.Kubernetes.Version.Status),
		})
	}

	return utils.NilOp()
}

func (t TenantControlPlaneTelemetry) OnUpdate(newObject runtime.Object, prevObject runtime.Object) AdmissionResponse {
	if t.Enabled {
		prevTCP, newTCP := prevObject.(*kamajiv1alpha1.TenantControlPlane), newObject.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		go t.TelemetryClient.PushUpdate(context.Background(), api.Update{
			KamajiVersion:     t.KamajiVersion,
			KubernetesVersion: t.KubernetesVersion,
			OldTenantVersion:  prevTCP.Status.Kubernetes.Version.Version,
			NewTenantVersion:  newTCP.Spec.Kubernetes.Version,
		})
	}

	return utils.NilOp()
}

func (t TenantControlPlaneTelemetry) extractTCPVersion(status *kamajiv1alpha1.KubernetesVersionStatus) string {
	if status == nil {
		return "Unknown"
	}

	return string(*status)
}
