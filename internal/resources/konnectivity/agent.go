// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/constants"
	"github.com/clastix/kamaji/internal/utilities"
)

const (
	agentNamespace = "kube-system"
)

type Agent struct {
	resource     *appsv1.DaemonSet
	Client       client.Client
	Name         string
	tenantClient client.Client
}

func (r *Agent) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Addons.Konnectivity.Agent.Checksum != r.resource.GetAnnotations()[constants.Checksum]
}

func (r *Agent) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Spec.Addons.Konnectivity == nil
}

func (r *Agent) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	if err := r.tenantClient.Delete(ctx, r.resource); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (r *Agent) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (err error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	r.resource = &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AgentName,
			Namespace: agentNamespace,
		},
	}

	if r.tenantClient, err = utilities.GetTenantClient(ctx, r.Client, tenantControlPlane); err != nil {
		logger.Error(err, "unable to retrieve the Tenant Control Plane client")

		return err
	}

	return nil
}

func (r *Agent) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return controllerutil.CreateOrUpdate(ctx, r.tenantClient, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *Agent) GetName() string {
	return r.Name
}

func (r *Agent) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Spec.Addons.Konnectivity != nil {
		tenantControlPlane.Status.Addons.Konnectivity.Agent = kamajiv1alpha1.ExternalKubernetesObjectStatus{
			Name:       r.resource.GetName(),
			Namespace:  r.resource.GetNamespace(),
			Checksum:   r.resource.GetAnnotations()[constants.Checksum],
			LastUpdate: metav1.Now(),
		}
		tenantControlPlane.Status.Addons.Konnectivity.Enabled = true

		return nil
	}

	tenantControlPlane.Status.Addons.Konnectivity.Enabled = false
	tenantControlPlane.Status.Addons.Konnectivity.Agent = kamajiv1alpha1.ExternalKubernetesObjectStatus{}

	return nil
}

func (r *Agent) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())

		address, _, err := tenantControlPlane.AssignedControlPlaneAddress()
		if err != nil {
			logger.Error(err, "unable to retrieve the Tenant Control Plane address")

			return err
		}

		r.resource.SetLabels(utilities.MergeMaps(
			utilities.KamajiLabels(),
			map[string]string{
				"k8s-app":                         AgentName,
				"addonmanager.kubernetes.io/mode": "Reconcile",
			},
		))

		if r.resource.Spec.Selector == nil {
			r.resource.Spec.Selector = &metav1.LabelSelector{}
		}
		r.resource.Spec.Selector.MatchLabels = map[string]string{
			"k8s-app": AgentName,
		}

		r.resource.Spec.Template.SetLabels(utilities.MergeMaps(
			r.resource.Spec.Template.GetLabels(),
			map[string]string{
				"k8s-app": AgentName,
			},
		))

		r.resource.Spec.Template.Spec.PriorityClassName = "system-cluster-critical"
		r.resource.Spec.Template.Spec.Tolerations = []corev1.Toleration{
			{
				Key:      "CriticalAddonsOnly",
				Operator: "Exists",
			},
		}
		r.resource.Spec.Template.Spec.NodeSelector = map[string]string{
			"kubernetes.io/os": "linux",
		}
		r.resource.Spec.Template.Spec.ServiceAccountName = AgentName
		r.resource.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: agentTokenName,
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{
								ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
									Path:              agentTokenName,
									Audience:          tenantControlPlane.Status.Addons.Konnectivity.ClusterRoleBinding.Name,
									ExpirationSeconds: pointer.Int64(3600),
								},
							},
						},
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
		}

		if len(r.resource.Spec.Template.Spec.Containers) != 1 {
			r.resource.Spec.Template.Spec.Containers = make([]corev1.Container, 1)
		}

		r.resource.Spec.Template.Spec.Containers[0].Image = fmt.Sprintf("%s:%s", tenantControlPlane.Spec.Addons.Konnectivity.AgentImage, tenantControlPlane.Spec.Addons.Konnectivity.Version)
		r.resource.Spec.Template.Spec.Containers[0].Name = AgentName
		r.resource.Spec.Template.Spec.Containers[0].Command = []string{"/proxy-agent"}
		r.resource.Spec.Template.Spec.Containers[0].Args = []string{
			"-v=8",
			"--logtostderr=true",
			"--ca-cert=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
			fmt.Sprintf("--proxy-server-host=%s", address),
			fmt.Sprintf("--proxy-server-port=%d", tenantControlPlane.Spec.Addons.Konnectivity.ProxyPort),
			"--admin-server-port=8133",
			"--health-server-port=8134",
			"--service-account-token-path=/var/run/secrets/tokens/konnectivity-agent-token",
		}
		r.resource.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				MountPath: "/var/run/secrets/tokens",
				Name:      agentTokenName,
			},
		}
		r.resource.Spec.Template.Spec.Containers[0].LivenessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/healthz",
					Port:   intstr.FromInt(8134),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 15,
			TimeoutSeconds:      15,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		}
		// Creating a copy to remove the metadata that would be changed at every reconciliation
		c := r.resource.DeepCopy()
		c.SetAnnotations(nil)
		c.SetResourceVersion("")

		yaml, _ := utilities.EncodeToYaml(c)
		annotations := r.resource.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[constants.Checksum] = utilities.MD5Checksum(yaml)
		r.resource.SetAnnotations(annotations)

		return nil
	}
}
