// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	pointer "k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/constants"
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/utilities"
)

type Agent struct {
	resource     client.Object
	Client       client.Client
	tenantClient client.Client
}

func (r *Agent) GetHistogram() prometheus.Histogram {
	agentCollector = resources.LazyLoadHistogramFromResource(agentCollector, r)

	return agentCollector
}

func (r *Agent) ShouldStatusBeUpdated(_ context.Context, tcp *kamajiv1alpha1.TenantControlPlane) bool {
	return tcp.Spec.Addons.Konnectivity == nil && (tcp.Status.Addons.Konnectivity.Agent.Namespace != "" || tcp.Status.Addons.Konnectivity.Agent.Name != "") ||
		tcp.Spec.Addons.Konnectivity != nil && (tcp.Status.Addons.Konnectivity.Agent.Namespace != r.resource.GetNamespace() || tcp.Status.Addons.Konnectivity.Agent.Name != r.resource.GetName()) ||
		tcp.Spec.Addons.Konnectivity.KonnectivityAgentSpec.Mode != tcp.Status.Addons.Konnectivity.Agent.Mode
}

func (r *Agent) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Spec.Addons.Konnectivity == nil && tenantControlPlane.Status.Addons.Konnectivity.Enabled
}

func (r *Agent) CleanUp(ctx context.Context, _ *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	if err := r.tenantClient.Get(ctx, client.ObjectKeyFromObject(r.resource), r.resource); err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}

		logger.Error(err, "cannot retrieve the requested resource for deletion")

		return false, err
	}

	if labels := r.resource.GetLabels(); labels == nil || labels[constants.ProjectNameLabelKey] != constants.ProjectNameLabelValue {
		return false, nil
	}

	if err := r.tenantClient.Delete(ctx, r.resource); err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}

		logger.Error(err, "cannot delete the requested resource")

		return false, err
	}

	return true, nil
}

func (r *Agent) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (err error) {
	logger := log.FromContext(ctx, "resource", r.GetName())

	switch tenantControlPlane.Spec.Addons.Konnectivity.KonnectivityAgentSpec.Mode {
	case kamajiv1alpha1.KonnectivityAgentModeDaemonSet:
		r.resource = &appsv1.DaemonSet{}
	case kamajiv1alpha1.KonnectivityAgentModeDeployment:
		r.resource = &appsv1.Deployment{}
	default:
		logger.Info("TenantControlPlane CRD is not updated, or validation failed, fallback to DaemonSet")

		r.resource = &appsv1.DaemonSet{}
	}

	r.resource.SetNamespace(AgentNamespace)
	r.resource.SetName(AgentName)

	if r.tenantClient, err = utilities.GetTenantClient(ctx, r.Client, tenantControlPlane); err != nil {
		logger.Error(err, "unable to retrieve the Tenant Control Plane client")

		return err
	}

	return nil
}

func (r *Agent) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if tenantControlPlane.Spec.Addons.Konnectivity != nil {
		or, err := controllerutil.CreateOrUpdate(ctx, r.tenantClient, r.resource, r.mutate(ctx, tenantControlPlane))
		if err != nil {
			return controllerutil.OperationResultNone, err
		}

		switch {
		case tenantControlPlane.Spec.Addons.Konnectivity.KonnectivityAgentSpec.Mode == kamajiv1alpha1.KonnectivityAgentModeDaemonSet &&
			tenantControlPlane.Status.Addons.Konnectivity.Agent.Mode != kamajiv1alpha1.KonnectivityAgentModeDaemonSet:
			var obj appsv1.Deployment
			obj.SetName(r.resource.GetName())
			obj.SetNamespace(r.resource.GetNamespace())

			if cleanupErr := r.tenantClient.Delete(ctx, &obj); cleanupErr != nil {
				log.FromContext(ctx, "resource", r.GetName()).Error(cleanupErr, "cannot cleanup older appsv1.Deployment")
			}
		case tenantControlPlane.Spec.Addons.Konnectivity.KonnectivityAgentSpec.Mode == kamajiv1alpha1.KonnectivityAgentModeDeployment &&
			tenantControlPlane.Status.Addons.Konnectivity.Agent.Mode != kamajiv1alpha1.KonnectivityAgentModeDeployment:
			var obj appsv1.DaemonSet
			obj.SetName(r.resource.GetName())
			obj.SetNamespace(r.resource.GetNamespace())

			if cleanupErr := r.tenantClient.Delete(ctx, &obj); cleanupErr != nil {
				log.FromContext(ctx, "resource", r.GetName()).Error(cleanupErr, "cannot cleanup older appsv1.DaemonSet")
			}
		}

		return or, nil
	}

	return controllerutil.OperationResultNone, nil
}

func (r *Agent) GetName() string {
	return "konnectivity-agent"
}

func (r *Agent) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.Addons.Konnectivity.Agent = kamajiv1alpha1.KonnectivityAgentStatus{}

	if tenantControlPlane.Spec.Addons.Konnectivity != nil {
		tenantControlPlane.Status.Addons.Konnectivity.Agent = kamajiv1alpha1.KonnectivityAgentStatus{
			ExternalKubernetesObjectStatus: kamajiv1alpha1.ExternalKubernetesObjectStatus{
				Name:       r.resource.GetName(),
				Namespace:  r.resource.GetNamespace(),
				LastUpdate: metav1.Now(),
			},
			Mode: tenantControlPlane.Spec.Addons.Konnectivity.KonnectivityAgentSpec.Mode,
		}
	}

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

		r.resource.SetLabels(utilities.MergeMaps(r.resource.GetLabels(), utilities.KamajiLabels(tenantControlPlane.GetName(), r.GetName())))

		specSelector := &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"k8s-app": AgentName,
			},
		}

		var podTemplateSpec *corev1.PodTemplateSpec

		switch obj := r.resource.(type) {
		case *appsv1.DaemonSet:
			obj.Spec.Selector = specSelector
			podTemplateSpec = &obj.Spec.Template
		case *appsv1.Deployment:
			obj.Spec.Selector = specSelector
			podTemplateSpec = &obj.Spec.Template
		}

		podTemplateSpec.SetLabels(utilities.MergeMaps(podTemplateSpec.GetLabels(), specSelector.MatchLabels))
		podTemplateSpec.Spec.PriorityClassName = "system-cluster-critical"
		podTemplateSpec.Spec.Tolerations = tenantControlPlane.Spec.Addons.Konnectivity.KonnectivityAgentSpec.Tolerations
		podTemplateSpec.Spec.NodeSelector = map[string]string{
			"kubernetes.io/os": "linux",
		}
		podTemplateSpec.Spec.ServiceAccountName = AgentName
		podTemplateSpec.Spec.Volumes = []corev1.Volume{
			{
				Name: agentTokenName,
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{
								ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
									Path:              agentTokenName,
									Audience:          tenantControlPlane.Status.Addons.Konnectivity.ClusterRoleBinding.Name,
									ExpirationSeconds: pointer.To(int64(3600)),
								},
							},
						},
						DefaultMode: pointer.To(int32(420)),
					},
				},
			},
		}

		if len(podTemplateSpec.Spec.Containers) != 1 {
			podTemplateSpec.Spec.Containers = make([]corev1.Container, 1)
		}

		podTemplateSpec.Spec.Containers[0].Image = fmt.Sprintf("%s:%s", tenantControlPlane.Spec.Addons.Konnectivity.KonnectivityAgentSpec.Image, tenantControlPlane.Spec.Addons.Konnectivity.KonnectivityAgentSpec.Version)
		podTemplateSpec.Spec.Containers[0].Name = AgentName
		podTemplateSpec.Spec.Containers[0].Command = []string{"/proxy-agent"}
		podTemplateSpec.Spec.HostNetwork = tenantControlPlane.Spec.Addons.Konnectivity.KonnectivityAgentSpec.UseHostNetwork

		args := make(map[string]string)
		args["-v"] = "8"
		args["--logtostderr"] = "true"
		args["--ca-cert"] = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
		args["--proxy-server-host"] = address
		args["--proxy-server-port"] = fmt.Sprintf("%d", tenantControlPlane.Spec.Addons.Konnectivity.KonnectivityServerSpec.Port)
		args["--admin-server-port"] = "8133"
		args["--health-server-port"] = "8134"
		args["--service-account-token-path"] = "/var/run/secrets/tokens/konnectivity-agent-token"

		extraArgs := utilities.ArgsFromSliceToMap(tenantControlPlane.Spec.Addons.Konnectivity.KonnectivityAgentSpec.ExtraArgs)

		for k, v := range extraArgs {
			args[k] = v
		}

		podTemplateSpec.Spec.Containers[0].Args = utilities.ArgsFromMapToSlice(args)
		podTemplateSpec.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				MountPath: "/var/run/secrets/tokens",
				Name:      agentTokenName,
			},
		}
		podTemplateSpec.Spec.Containers[0].LivenessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/healthz",
					Port:   intstr.FromInt32(8134),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 15,
			TimeoutSeconds:      15,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		}

		switch tenantControlPlane.Spec.Addons.Konnectivity.KonnectivityAgentSpec.Mode {
		case kamajiv1alpha1.KonnectivityAgentModeDaemonSet:
			r.resource.(*appsv1.DaemonSet).Spec.Template = *podTemplateSpec //nolint:forcetypeassert
		case kamajiv1alpha1.KonnectivityAgentModeDeployment:
			//nolint:forcetypeassert
			r.resource.(*appsv1.Deployment).Spec.Template = *podTemplateSpec
			//nolint:forcetypeassert
			r.resource.(*appsv1.Deployment).Spec.Replicas = pointer.To(tenantControlPlane.Spec.Addons.Konnectivity.KonnectivityAgentSpec.Replicas)
		}

		return nil
	}
}
