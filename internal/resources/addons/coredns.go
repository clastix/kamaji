// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"bytes"
	"context"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/constants"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/resources"
	addons_utils "github.com/clastix/kamaji/internal/resources/addons/utils"
	"github.com/clastix/kamaji/internal/resources/utils"
	"github.com/clastix/kamaji/internal/utilities"
)

type CoreDNS struct {
	Client client.Client

	deployment         *appsv1.Deployment
	configMap          *corev1.ConfigMap
	service            *corev1.Service
	clusterRole        *rbacv1.ClusterRole
	clusterRoleBinding *rbacv1.ClusterRoleBinding
	serviceAccount     *corev1.ServiceAccount
}

func (c *CoreDNS) GetHistogram() prometheus.Histogram {
	coreDNSCollector = resources.LazyLoadHistogramFromResource(coreDNSCollector, c)

	return coreDNSCollector
}

func (c *CoreDNS) Define(context.Context, *kamajiv1alpha1.TenantControlPlane) error {
	c.deployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeadm.CoreDNSName,
			Namespace: kubeadm.KubeSystemNamespace,
		},
	}
	c.configMap = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeadm.CoreDNSName,
			Namespace: kubeadm.KubeSystemNamespace,
		},
	}
	c.service = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeadm.CoreDNSServiceName,
			Namespace: kubeadm.KubeSystemNamespace,
		},
	}
	c.clusterRole = &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: kubeadm.CoreDNSClusterRoleName,
		},
	}
	c.clusterRoleBinding = &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: kubeadm.CoreDNSClusterRoleBindingName,
		},
	}
	c.serviceAccount = &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeadm.CoreDNSName,
			Namespace: kubeadm.KubeSystemNamespace,
		},
	}

	return nil
}

func (c *CoreDNS) ShouldCleanup(tcp *kamajiv1alpha1.TenantControlPlane) bool {
	return tcp.Spec.Addons.CoreDNS == nil && tcp.Status.Addons.CoreDNS.Enabled
}

func (c *CoreDNS) CleanUp(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", "kubeadm_addons", "addon", c.GetName())

	tenantClient, err := utilities.GetTenantClient(ctx, c.Client, tcp)
	if err != nil {
		logger.Error(err, "cannot generate Tenant client")

		return false, err
	}

	var deleted bool

	for _, obj := range []client.Object{c.serviceAccount, c.clusterRoleBinding, c.clusterRole, c.service, c.configMap, c.deployment} {
		objectKey := client.ObjectKeyFromObject(obj)

		if err = tenantClient.Get(ctx, objectKey, obj); err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}
		}
		// Don't delete resource if it is not managed by Kamaji
		if labels := obj.GetLabels(); labels == nil || labels[constants.ProjectNameLabelKey] != constants.ProjectNameLabelValue {
			continue
		}

		if err = tenantClient.Delete(ctx, obj); err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}

			return false, err
		}

		deleted = true
	}

	return deleted, nil
}

func (c *CoreDNS) CreateOrUpdate(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	logger := log.FromContext(ctx, "addon", c.GetName())

	if tcp.Spec.Addons.CoreDNS == nil {
		return controllerutil.OperationResultNone, nil
	}

	tenantClient, err := utilities.GetTenantClient(ctx, c.Client, tcp)
	if err != nil {
		logger.Error(err, "cannot generate Tenant client")

		return controllerutil.OperationResultNone, err
	}

	if err = c.decodeManifests(ctx, tcp); err != nil {
		logger.Error(err, "manifest decoding failed")

		return controllerutil.OperationResultNone, err
	}

	var operationResult controllerutil.OperationResult

	reconciliationResult := controllerutil.OperationResultNone
	// ClusterRoleBinding
	operationResult, err = c.mutateClusterRoleBinding(ctx, tenantClient)
	if err != nil {
		logger.Error(err, "ClusterRoleBinding reconciliation failed")

		return controllerutil.OperationResultNone, err
	}
	reconciliationResult = utils.UpdateOperationResult(reconciliationResult, operationResult)
	// Deployment
	operationResult, err = c.mutateDeployment(ctx, tenantClient)
	if err != nil {
		logger.Error(err, "Deployment reconciliation failed")

		return controllerutil.OperationResultNone, err
	}
	reconciliationResult = utils.UpdateOperationResult(reconciliationResult, operationResult)
	// ConfigMap
	operationResult, err = c.mutateConfigMap(ctx, tenantClient)
	if err != nil {
		logger.Error(err, "ConfigMap reconciliation failed")

		return controllerutil.OperationResultNone, err
	}
	reconciliationResult = utils.UpdateOperationResult(reconciliationResult, operationResult)
	// Service
	operationResult, err = c.mutateService(ctx, tenantClient)
	if err != nil {
		logger.Error(err, "Service reconciliation failed")

		return controllerutil.OperationResultNone, err
	}
	reconciliationResult = utils.UpdateOperationResult(reconciliationResult, operationResult)
	// ClusterRole
	operationResult, err = c.mutateClusterRole(ctx, tenantClient)
	if err != nil {
		logger.Error(err, "ClusterRole reconciliation failed")

		return controllerutil.OperationResultNone, err
	}
	reconciliationResult = utils.UpdateOperationResult(reconciliationResult, operationResult)
	// ServiceAccount
	operationResult, err = c.mutateServiceAccount(ctx, tenantClient)
	if err != nil {
		logger.Error(err, "ServiceAccount reconciliation failed")

		return controllerutil.OperationResultNone, err
	}
	reconciliationResult = utils.UpdateOperationResult(reconciliationResult, operationResult)

	return reconciliationResult, nil
}

func (c *CoreDNS) GetName() string {
	return "coredns"
}

func (c *CoreDNS) ShouldStatusBeUpdated(_ context.Context, tcp *kamajiv1alpha1.TenantControlPlane) bool {
	return tcp.Spec.Addons.CoreDNS != nil && !tcp.Status.Addons.CoreDNS.Enabled
}

func (c *CoreDNS) UpdateTenantControlPlaneStatus(_ context.Context, tcp *kamajiv1alpha1.TenantControlPlane) error {
	tcp.Status.Addons.CoreDNS.Enabled = tcp.Spec.Addons.CoreDNS != nil
	tcp.Status.Addons.CoreDNS.LastUpdate = metav1.Now()

	return nil
}

func (c *CoreDNS) decodeManifests(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) error {
	tcpClient, config, err := resources.GetKubeadmManifestDeps(ctx, c.Client, tcp)
	if err != nil {
		return errors.Wrap(err, "unable to create manifests dependencies")
	}

	// If CoreDNS addon is enabled and with an override, adding these to the kubeadm init configuration
	config.Parameters.CoreDNSOptions = &kubeadm.AddonOptions{}

	if len(tcp.Spec.Addons.CoreDNS.ImageRepository) > 0 {
		config.Parameters.CoreDNSOptions.Repository = tcp.Spec.Addons.CoreDNS.ImageRepository
	}

	if len(tcp.Spec.Addons.CoreDNS.ImageRepository) > 0 {
		config.Parameters.CoreDNSOptions.Tag = tcp.Spec.Addons.CoreDNS.ImageTag
	}

	manifests, err := kubeadm.AddCoreDNS(tcpClient, config)
	if err != nil {
		return errors.Wrap(err, "unable to generate manifests")
	}

	parts := bytes.Split(manifests, []byte("---"))

	if err = utilities.DecodeFromYAML(string(parts[1]), c.deployment); err != nil {
		return errors.Wrap(err, "unable to decode Deployment manifest")
	}
	addons_utils.SetKamajiManagedLabels(c.deployment)

	if err = utilities.DecodeFromYAML(string(parts[2]), c.configMap); err != nil {
		return errors.Wrap(err, "unable to decode ConfigMap manifest")
	}
	addons_utils.SetKamajiManagedLabels(c.configMap)

	if err = utilities.DecodeFromYAML(string(parts[3]), c.service); err != nil {
		return errors.Wrap(err, "unable to decode Service manifest")
	}
	addons_utils.SetKamajiManagedLabels(c.service)

	if err = utilities.DecodeFromYAML(string(parts[4]), c.clusterRole); err != nil {
		return errors.Wrap(err, "unable to decode ClusterRole manifest")
	}
	addons_utils.SetKamajiManagedLabels(c.clusterRole)

	if err = utilities.DecodeFromYAML(string(parts[5]), c.clusterRoleBinding); err != nil {
		return errors.Wrap(err, "unable to decode ClusterRoleBinding manifest")
	}
	addons_utils.SetKamajiManagedLabels(c.clusterRoleBinding)

	if err = utilities.DecodeFromYAML(string(parts[6]), c.serviceAccount); err != nil {
		return errors.Wrap(err, "unable to decode ServiceAccount manifest")
	}
	addons_utils.SetKamajiManagedLabels(c.serviceAccount)

	return nil
}

func (c *CoreDNS) mutateClusterRoleBinding(ctx context.Context, tenantClient client.Client) (controllerutil.OperationResult, error) {
	crb := &rbacv1.ClusterRoleBinding{}
	crb.SetName(c.clusterRoleBinding.GetName())

	defer func() {
		c.clusterRoleBinding.SetUID(crb.GetUID())
	}()

	return utilities.CreateOrUpdateWithConflict(ctx, tenantClient, crb, func() error {
		crb.SetLabels(utilities.MergeMaps(crb.GetLabels(), c.clusterRoleBinding.GetLabels()))
		crb.SetAnnotations(utilities.MergeMaps(crb.GetAnnotations(), c.clusterRoleBinding.GetAnnotations()))
		crb.Subjects = c.clusterRoleBinding.Subjects
		crb.RoleRef = c.clusterRoleBinding.RoleRef

		return nil
	})
}

func (c *CoreDNS) mutateDeployment(ctx context.Context, tenantClient client.Client) (controllerutil.OperationResult, error) {
	d := &appsv1.Deployment{}
	d.SetName(c.deployment.GetName())
	d.SetNamespace(c.deployment.GetNamespace())

	return utilities.CreateOrUpdateWithConflict(ctx, tenantClient, d, func() error {
		d.SetLabels(utilities.MergeMaps(d.GetLabels(), c.deployment.GetLabels()))
		d.SetAnnotations(utilities.MergeMaps(d.GetAnnotations(), c.deployment.GetAnnotations()))
		d.Spec.Replicas = c.deployment.Spec.Replicas
		d.Spec.Selector = c.deployment.Spec.Selector
		d.Spec.Template.Labels = c.deployment.Spec.Selector.MatchLabels
		if len(d.Spec.Template.Spec.Volumes) != 1 {
			d.Spec.Template.Spec.Volumes = make([]corev1.Volume, 1)
		}
		d.Spec.Template.Spec.Volumes[0].Name = c.deployment.Spec.Template.Spec.Volumes[0].Name
		if d.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap == nil {
			d.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap = &corev1.ConfigMapVolumeSource{}
		}
		d.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap.LocalObjectReference.Name = c.deployment.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap.LocalObjectReference.Name
		if len(d.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap.Items) == 0 {
			d.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap.Items = make([]corev1.KeyToPath, 1)
		}
		d.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap.Items[0].Key = c.deployment.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap.Items[0].Key
		d.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap.Items[0].Path = c.deployment.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap.Items[0].Path
		if len(d.Spec.Template.Spec.Containers) == 0 {
			d.Spec.Template.Spec.Containers = make([]corev1.Container, 1)
		}
		d.Spec.Template.Spec.Containers[0].Name = c.deployment.Spec.Template.Spec.Containers[0].Name
		d.Spec.Template.Spec.Containers[0].Image = c.deployment.Spec.Template.Spec.Containers[0].Image
		d.Spec.Template.Spec.Containers[0].Args = c.deployment.Spec.Template.Spec.Containers[0].Args
		if len(d.Spec.Template.Spec.Containers[0].Ports) != 3 {
			d.Spec.Template.Spec.Containers[0].Ports = make([]corev1.ContainerPort, 3)
		}
		d.Spec.Template.Spec.Containers[0].Ports[0].Name = c.deployment.Spec.Template.Spec.Containers[0].Ports[0].Name
		d.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = c.deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort
		d.Spec.Template.Spec.Containers[0].Ports[0].Protocol = c.deployment.Spec.Template.Spec.Containers[0].Ports[0].Protocol
		d.Spec.Template.Spec.Containers[0].Ports[1].Name = c.deployment.Spec.Template.Spec.Containers[0].Ports[1].Name
		d.Spec.Template.Spec.Containers[0].Ports[1].ContainerPort = c.deployment.Spec.Template.Spec.Containers[0].Ports[1].ContainerPort
		d.Spec.Template.Spec.Containers[0].Ports[1].Protocol = c.deployment.Spec.Template.Spec.Containers[0].Ports[1].Protocol
		d.Spec.Template.Spec.Containers[0].Ports[2].Name = c.deployment.Spec.Template.Spec.Containers[0].Ports[2].Name
		d.Spec.Template.Spec.Containers[0].Ports[2].ContainerPort = c.deployment.Spec.Template.Spec.Containers[0].Ports[2].ContainerPort
		d.Spec.Template.Spec.Containers[0].Ports[2].Protocol = c.deployment.Spec.Template.Spec.Containers[0].Ports[2].Protocol
		d.Spec.Template.Spec.Containers[0].Resources = c.deployment.Spec.Template.Spec.Containers[0].Resources
		if len(d.Spec.Template.Spec.Containers[0].VolumeMounts) == 0 {
			d.Spec.Template.Spec.Containers[0].VolumeMounts = make([]corev1.VolumeMount, 1)
		}
		d.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name = c.deployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name
		d.Spec.Template.Spec.Containers[0].VolumeMounts[0].ReadOnly = c.deployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].ReadOnly
		d.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath = c.deployment.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath
		if d.Spec.Template.Spec.Containers[0].LivenessProbe == nil {
			d.Spec.Template.Spec.Containers[0].LivenessProbe = &corev1.Probe{}
		}
		d.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet = c.deployment.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet
		d.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds = c.deployment.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds
		d.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds = c.deployment.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds
		d.Spec.Template.Spec.Containers[0].LivenessProbe.SuccessThreshold = c.deployment.Spec.Template.Spec.Containers[0].LivenessProbe.SuccessThreshold
		d.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold = c.deployment.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold
		if d.Spec.Template.Spec.Containers[0].ReadinessProbe == nil {
			d.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{}
		}
		d.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet = c.deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet
		if d.Spec.Template.Spec.Containers[0].SecurityContext == nil {
			d.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{}
		}
		d.Spec.Template.Spec.Containers[0].SecurityContext.Capabilities = c.deployment.Spec.Template.Spec.Containers[0].SecurityContext.Capabilities
		d.Spec.Template.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem = c.deployment.Spec.Template.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem
		d.Spec.Template.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation = c.deployment.Spec.Template.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation
		d.Spec.Template.Spec.DNSPolicy = c.deployment.Spec.Template.Spec.DNSPolicy
		d.Spec.Template.Spec.NodeSelector = c.deployment.Spec.Template.Spec.NodeSelector
		d.Spec.Template.Spec.ServiceAccountName = c.deployment.Spec.Template.Spec.ServiceAccountName
		if d.Spec.Template.Spec.Affinity == nil {
			d.Spec.Template.Spec.Affinity = &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{},
			}
		}
		d.Spec.Template.Spec.Affinity.PodAffinity = c.deployment.Spec.Template.Spec.Affinity.PodAffinity
		d.Spec.Template.Spec.Tolerations = c.deployment.Spec.Template.Spec.Tolerations
		d.Spec.Template.Spec.PriorityClassName = c.deployment.Spec.Template.Spec.PriorityClassName
		d.Spec.Strategy.Type = c.deployment.Spec.Strategy.Type

		return controllerutil.SetControllerReference(c.clusterRoleBinding, d, tenantClient.Scheme())
	})
}

func (c *CoreDNS) mutateConfigMap(ctx context.Context, tenantClient client.Client) (controllerutil.OperationResult, error) {
	cm := &corev1.ConfigMap{}
	cm.SetName(c.configMap.GetName())
	cm.SetNamespace(c.configMap.GetNamespace())

	return utilities.CreateOrUpdateWithConflict(ctx, tenantClient, cm, func() error {
		cm.SetLabels(utilities.MergeMaps(cm.GetLabels(), c.configMap.GetLabels()))
		cm.SetAnnotations(utilities.MergeMaps(cm.GetAnnotations(), c.configMap.GetAnnotations()))
		cm.Data = c.configMap.Data

		return controllerutil.SetControllerReference(c.clusterRoleBinding, cm, tenantClient.Scheme())
	})
}

func (c *CoreDNS) mutateService(ctx context.Context, tenantClient client.Client) (controllerutil.OperationResult, error) {
	svc := &corev1.Service{}
	svc.SetName(c.service.GetName())
	svc.SetNamespace(c.service.GetNamespace())

	return utilities.CreateOrUpdateWithConflict(ctx, tenantClient, svc, func() error {
		svc.SetLabels(utilities.MergeMaps(svc.GetLabels(), c.service.GetLabels()))
		svc.SetAnnotations(utilities.MergeMaps(svc.GetAnnotations(), c.service.GetAnnotations()))

		svc.Spec.Ports = c.service.Spec.Ports
		svc.Spec.Selector = c.service.Spec.Selector
		svc.Spec.ClusterIP = c.service.Spec.ClusterIP

		return controllerutil.SetControllerReference(c.clusterRoleBinding, svc, tenantClient.Scheme())
	})
}

func (c *CoreDNS) mutateClusterRole(ctx context.Context, tenantClient client.Client) (controllerutil.OperationResult, error) {
	cr := &rbacv1.ClusterRole{}
	cr.SetName(c.clusterRole.GetName())
	cr.SetNamespace(c.clusterRole.GetNamespace())

	return utilities.CreateOrUpdateWithConflict(ctx, tenantClient, cr, func() error {
		cr.SetLabels(utilities.MergeMaps(cr.GetLabels(), c.clusterRole.GetLabels()))
		cr.SetAnnotations(utilities.MergeMaps(cr.GetAnnotations(), c.clusterRole.GetAnnotations()))
		cr.Rules = c.clusterRole.Rules

		return controllerutil.SetControllerReference(c.clusterRoleBinding, cr, tenantClient.Scheme())
	})
}

func (c *CoreDNS) mutateServiceAccount(ctx context.Context, tenantClient client.Client) (controllerutil.OperationResult, error) {
	sa := &corev1.ServiceAccount{}
	sa.SetName(c.serviceAccount.GetName())
	sa.SetNamespace(c.serviceAccount.GetNamespace())

	return utilities.CreateOrUpdateWithConflict(ctx, tenantClient, sa, func() error {
		sa.SetLabels(utilities.MergeMaps(sa.GetLabels(), c.serviceAccount.GetLabels()))
		sa.SetAnnotations(utilities.MergeMaps(sa.GetAnnotations(), c.serviceAccount.GetAnnotations()))

		return controllerutil.SetControllerReference(c.clusterRoleBinding, sa, tenantClient.Scheme())
	})
}
