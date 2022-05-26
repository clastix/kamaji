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

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type Agent struct {
	resource     *appsv1.DaemonSet
	Client       client.Client
	Name         string
	tenantClient client.Client
}

func (r *Agent) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.Addons.Konnectivity.Agent.Name != r.resource.GetName() ||
		tenantControlPlane.Status.Addons.Konnectivity.Agent.Namespace != r.resource.GetNamespace() ||
		tenantControlPlane.Status.Addons.Konnectivity.Agent.RV != r.resource.ObjectMeta.ResourceVersion
}

func (r *Agent) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Spec.Addons.Konnectivity != nil
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

func (r *Agent) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AgentName,
			Namespace: kubeSystemNamespace,
		},
	}

	client, err := NewClient(ctx, r, tenantControlPlane)
	if err != nil {
		return err
	}

	r.tenantClient = client

	return nil
}

func (r *Agent) GetClient() client.Client {
	return r.Client
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
			RV:         r.resource.ObjectMeta.ResourceVersion,
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
		address := tenantControlPlane.Spec.Addons.Konnectivity.ProxyHost
		if address == "" {
			address = tenantControlPlane.Spec.NetworkProfile.Address
		}

		r.resource.SetLabels(utilities.MergeMaps(
			utilities.KamajiLabels(),
			map[string]string{
				"k8s-app":                         AgentName,
				"addonmanager.kubernetes.io/mode": "Reconcile",
			},
		))

		r.resource.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"k8s-app": AgentName,
			},
		}

		r.resource.Spec.Template.SetLabels(utilities.MergeMaps(
			r.resource.Spec.Template.GetLabels(),
			map[string]string{
				"k8s-app": AgentName,
			},
		))

		r.resource.Spec.Template.Spec = corev1.PodSpec{
			PriorityClassName: "system-cluster-critical",
			Tolerations: []corev1.Toleration{
				{
					Key:      "CriticalAddonsOnly",
					Operator: "Exists",
				},
			},
			NodeSelector: map[string]string{
				"kubernetes.io/os": "linux",
			},
			Containers: []corev1.Container{
				{
					Image:   fmt.Sprintf("%s:%s", tenantControlPlane.Spec.Addons.Konnectivity.AgentImage, tenantControlPlane.Spec.Addons.Konnectivity.Version),
					Name:    AgentName,
					Command: []string{"/proxy-agent"},
					Args: []string{
						"-v=8",
						"--logtostderr=true",
						"--ca-cert=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
						fmt.Sprintf("--proxy-server-host=%s", address),
						fmt.Sprintf("--proxy-server-port=%d", tenantControlPlane.Spec.Addons.Konnectivity.ProxyPort),
						"--admin-server-port=8133",
						"--health-server-port=8134",
						"--service-account-token-path=/var/run/secrets/tokens/konnectivity-agent-token",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/var/run/secrets/tokens",
							Name:      agentTokenName,
						},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/healthz",
								Port: intstr.FromInt(8134),
							},
						},
						InitialDelaySeconds: 15,
						TimeoutSeconds:      15,
						PeriodSeconds:       10,
						SuccessThreshold:    1,
						FailureThreshold:    3,
					},
					TerminationMessagePath:   "/dev/termination-log",
					TerminationMessagePolicy: "File",
					ImagePullPolicy:          corev1.PullIfNotPresent,
				},
			},
			ServiceAccountName:            AgentName,
			DeprecatedServiceAccount:      AgentName,
			RestartPolicy:                 "Always",
			DNSPolicy:                     "ClusterFirst",
			TerminationGracePeriodSeconds: pointer.Int64(30),
			SchedulerName:                 "default-scheduler",
			SecurityContext:               &corev1.PodSecurityContext{},
			Volumes: []corev1.Volume{
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
			},
		}

		return nil
	}
}
