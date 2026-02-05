// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"bytes"
	"context"
	"fmt"

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
		return fmt.Errorf("unable to create manifests dependencies: %w", err)
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
		return fmt.Errorf("unable to generate manifests: %w", err)
	}

	parts := bytes.Split(manifests, []byte("---"))

	if err = utilities.DecodeFromYAML(string(parts[1]), c.deployment); err != nil {
		return fmt.Errorf("unable to decode Deployment manifest: %w", err)
	}
	addons_utils.SetKamajiManagedLabels(c.deployment)

	if err = utilities.DecodeFromYAML(string(parts[2]), c.configMap); err != nil {
		return fmt.Errorf("unable to decode ConfigMap manifest: %w", err)
	}
	addons_utils.SetKamajiManagedLabels(c.configMap)

	if err = utilities.DecodeFromYAML(string(parts[3]), c.service); err != nil {
		return fmt.Errorf("unable to decode Service manifest: %w", err)
	}
	addons_utils.SetKamajiManagedLabels(c.service)

	if err = utilities.DecodeFromYAML(string(parts[4]), c.clusterRole); err != nil {
		return fmt.Errorf("unable to decode ClusterRole manifest: %w", err)
	}
	addons_utils.SetKamajiManagedLabels(c.clusterRole)

	if err = utilities.DecodeFromYAML(string(parts[5]), c.clusterRoleBinding); err != nil {
		return fmt.Errorf("unable to decode ClusterRoleBinding manifest: %w", err)
	}
	addons_utils.SetKamajiManagedLabels(c.clusterRoleBinding)

	if err = utilities.DecodeFromYAML(string(parts[6]), c.serviceAccount); err != nil {
		return fmt.Errorf("unable to decode ServiceAccount manifest: %w", err)
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
	var deployment appsv1.Deployment
	deployment.Name = c.deployment.Name
	deployment.Namespace = c.deployment.Namespace

	if err := tenantClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment); err != nil {
		if k8serrors.IsNotFound(err) {
			return utilities.CreateOrUpdateWithConflict(ctx, tenantClient, c.deployment, func() error {
				return controllerutil.SetControllerReference(c.clusterRoleBinding, c.deployment, tenantClient.Scheme())
			})
		}

		return controllerutil.OperationResultNone, err
	}

	if err := controllerutil.SetControllerReference(c.clusterRoleBinding, c.deployment, tenantClient.Scheme()); err != nil {
		return controllerutil.OperationResultNone, err
	}

	return controllerutil.OperationResultNone, tenantClient.Patch(ctx, c.deployment, client.Apply, client.FieldOwner("kamaji"), client.ForceOwnership)
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
	var svc corev1.Service
	svc.Name = c.service.Name
	svc.Namespace = c.service.Namespace

	if err := tenantClient.Get(ctx, client.ObjectKeyFromObject(&svc), &svc); err != nil {
		if k8serrors.IsNotFound(err) {
			return utilities.CreateOrUpdateWithConflict(ctx, tenantClient, c.service, func() error {
				return controllerutil.SetControllerReference(c.clusterRoleBinding, c.service, tenantClient.Scheme())
			})
		}

		return controllerutil.OperationResultNone, err
	}

	if err := controllerutil.SetControllerReference(c.clusterRoleBinding, c.service, tenantClient.Scheme()); err != nil {
		return controllerutil.OperationResultNone, err
	}

	return controllerutil.OperationResultNone, tenantClient.Patch(ctx, c.service, client.Apply, client.FieldOwner("kamaji"), client.ForceOwnership)
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
