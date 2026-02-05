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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/constants"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/resources"
	addon_utils "github.com/clastix/kamaji/internal/resources/addons/utils"
	"github.com/clastix/kamaji/internal/resources/utils"
	"github.com/clastix/kamaji/internal/utilities"
)

type KubeProxy struct {
	Client client.Client

	serviceAccount     *corev1.ServiceAccount
	clusterRoleBinding *rbacv1.ClusterRoleBinding
	role               *rbacv1.Role
	roleBinding        *rbacv1.RoleBinding
	configMap          *corev1.ConfigMap
	daemonSet          *appsv1.DaemonSet
}

func (k *KubeProxy) GetHistogram() prometheus.Histogram {
	kubeProxyCollector = resources.LazyLoadHistogramFromResource(kubeProxyCollector, k)

	return kubeProxyCollector
}

func (k *KubeProxy) Define(context.Context, *kamajiv1alpha1.TenantControlPlane) error {
	k.clusterRoleBinding = &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: kubeadm.KubeProxyClusterRoleBindingName,
		},
	}
	k.serviceAccount = &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeadm.KubeProxyServiceAccountName,
			Namespace: kubeadm.KubeSystemNamespace,
		},
	}
	k.role = &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeadm.KubeProxyConfigMapRoleName,
			Namespace: kubeadm.KubeSystemNamespace,
		},
	}
	k.roleBinding = &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeadm.KubeProxyConfigMapRoleName,
			Namespace: kubeadm.KubeSystemNamespace,
		},
	}
	k.configMap = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeadm.KubeProxyConfigMap,
			Namespace: kubeadm.KubeSystemNamespace,
		},
	}
	k.daemonSet = &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeadm.KubeProxyName,
			Namespace: kubeadm.KubeSystemNamespace,
		},
	}

	return nil
}

func (k *KubeProxy) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Spec.Addons.KubeProxy == nil && tenantControlPlane.Status.Addons.KubeProxy.Enabled
}

func (k *KubeProxy) CleanUp(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	logger := log.FromContext(ctx, "resource", "kubeadm_addons", "addon", k.GetName())

	tenantClient, err := utilities.GetTenantClient(ctx, k.Client, tcp)
	if err != nil {
		logger.Error(err, "cannot generate Tenant client")

		return false, err
	}

	var deleted bool

	for _, obj := range []client.Object{k.serviceAccount, k.clusterRoleBinding, k.role, k.roleBinding, k.configMap, k.daemonSet} {
		if err = tenantClient.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, obj); err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}
		}
		// Skipping deletion:
		// the kubeproxy addons is not managed by Kamaji.
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

func (k *KubeProxy) CreateOrUpdate(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	if tcp.Spec.Addons.KubeProxy == nil {
		return controllerutil.OperationResultNone, nil
	}

	logger := log.FromContext(ctx, "addon", k.GetName())

	tenantClient, err := utilities.GetTenantClient(ctx, k.Client, tcp)
	if err != nil {
		logger.Error(err, "cannot generate Tenant client")

		return controllerutil.OperationResultNone, err
	}

	if err = k.decodeManifests(ctx, tcp); err != nil {
		logger.Error(err, "manifest decoding failed")

		return controllerutil.OperationResultNone, err
	}

	var operationResult controllerutil.OperationResult

	reconciliationResult := controllerutil.OperationResultNone
	// ClusterRoleBinding
	operationResult, err = k.mutateClusterRoleBinding(ctx, tenantClient)
	if err != nil {
		logger.Error(err, "ClusterRoleBinding reconciliation failed")

		return controllerutil.OperationResultNone, err
	}
	reconciliationResult = utils.UpdateOperationResult(reconciliationResult, operationResult)
	// DaemonSet
	operationResult, err = k.mutateDaemonSet(ctx, tenantClient)
	if err != nil {
		logger.Error(err, "DaemonSet reconciliation failed")

		return controllerutil.OperationResultNone, err
	}
	reconciliationResult = utils.UpdateOperationResult(reconciliationResult, operationResult)
	// ConfigMap
	operationResult, err = k.mutateConfigMap(ctx, tenantClient)
	if err != nil {
		logger.Error(err, "ConfigMap reconciliation failed")

		return controllerutil.OperationResultNone, err
	}
	reconciliationResult = utils.UpdateOperationResult(reconciliationResult, operationResult)
	// RoleBinding
	operationResult, err = k.mutateRoleBinding(ctx, tenantClient)
	if err != nil {
		logger.Error(err, "RoleBinding reconciliation failed")

		return controllerutil.OperationResultNone, err
	}
	reconciliationResult = utils.UpdateOperationResult(reconciliationResult, operationResult)
	// Role
	operationResult, err = k.mutateRole(ctx, tenantClient)
	if err != nil {
		logger.Error(err, "Role reconciliation failed")

		return controllerutil.OperationResultNone, err
	}
	reconciliationResult = utils.UpdateOperationResult(reconciliationResult, operationResult)
	// ServiceAccount
	operationResult, err = k.mutateServiceAccount(ctx, tenantClient)
	if err != nil {
		logger.Error(err, "ServiceAccount reconciliation failed")

		return controllerutil.OperationResultNone, err
	}
	reconciliationResult = utils.UpdateOperationResult(reconciliationResult, operationResult)

	return reconciliationResult, nil
}

func (k *KubeProxy) GetName() string {
	return "kube-proxy"
}

func (k *KubeProxy) ShouldStatusBeUpdated(_ context.Context, tcp *kamajiv1alpha1.TenantControlPlane) bool {
	return tcp.Spec.Addons.KubeProxy != nil && !tcp.Status.Addons.KubeProxy.Enabled
}

func (k *KubeProxy) UpdateTenantControlPlaneStatus(_ context.Context, tcp *kamajiv1alpha1.TenantControlPlane) error {
	tcp.Status.Addons.KubeProxy.Enabled = tcp.Spec.Addons.KubeProxy != nil
	tcp.Status.Addons.KubeProxy.LastUpdate = metav1.Now()

	return nil
}

func (k *KubeProxy) mutateClusterRoleBinding(ctx context.Context, tenantClient client.Client) (controllerutil.OperationResult, error) {
	crb := &rbacv1.ClusterRoleBinding{}
	crb.SetName(k.clusterRoleBinding.GetName())

	defer func() {
		k.clusterRoleBinding.SetUID(crb.GetUID())
	}()

	return utilities.CreateOrUpdateWithConflict(ctx, tenantClient, crb, func() error {
		crb.SetLabels(utilities.MergeMaps(crb.GetLabels(), k.clusterRoleBinding.GetLabels()))
		crb.SetAnnotations(utilities.MergeMaps(crb.GetAnnotations(), k.clusterRoleBinding.GetAnnotations()))
		crb.Subjects = k.clusterRoleBinding.Subjects
		crb.RoleRef = k.clusterRoleBinding.RoleRef

		return nil
	})
}

func (k *KubeProxy) mutateServiceAccount(ctx context.Context, tenantClient client.Client) (controllerutil.OperationResult, error) {
	sa := &corev1.ServiceAccount{}
	sa.SetName(k.serviceAccount.GetName())
	sa.SetNamespace(k.serviceAccount.GetNamespace())

	return utilities.CreateOrUpdateWithConflict(ctx, tenantClient, sa, func() error {
		sa.SetLabels(utilities.MergeMaps(sa.GetLabels(), k.serviceAccount.GetLabels()))
		sa.SetAnnotations(utilities.MergeMaps(sa.GetAnnotations(), k.serviceAccount.GetAnnotations()))

		return controllerutil.SetControllerReference(k.clusterRoleBinding, sa, tenantClient.Scheme())
	})
}

func (k *KubeProxy) mutateRole(ctx context.Context, tenantClient client.Client) (controllerutil.OperationResult, error) {
	r := &rbacv1.Role{}
	r.SetName(k.role.GetName())
	r.SetNamespace(k.role.GetNamespace())

	return utilities.CreateOrUpdateWithConflict(ctx, tenantClient, r, func() error {
		r.SetLabels(utilities.MergeMaps(r.GetLabels(), k.role.GetLabels()))
		r.SetAnnotations(utilities.MergeMaps(r.GetAnnotations(), k.role.GetAnnotations()))
		r.Rules = k.role.Rules

		return controllerutil.SetControllerReference(k.clusterRoleBinding, r, tenantClient.Scheme())
	})
}

func (k *KubeProxy) mutateRoleBinding(ctx context.Context, tenantClient client.Client) (controllerutil.OperationResult, error) {
	rb := &rbacv1.RoleBinding{}
	rb.SetName(k.roleBinding.GetName())
	rb.SetNamespace(k.roleBinding.GetNamespace())

	return utilities.CreateOrUpdateWithConflict(ctx, tenantClient, rb, func() error {
		rb.SetLabels(utilities.MergeMaps(rb.GetLabels(), k.roleBinding.GetLabels()))
		rb.SetAnnotations(utilities.MergeMaps(rb.GetAnnotations(), k.roleBinding.GetAnnotations()))
		if len(rb.Subjects) == 0 {
			rb.Subjects = make([]rbacv1.Subject, 1)
		}
		rb.Subjects[0].Kind = k.roleBinding.Subjects[0].Kind
		rb.Subjects[0].APIGroup = rbacv1.GroupName
		rb.Subjects[0].Name = k.roleBinding.Subjects[0].Name
		rb.RoleRef = k.roleBinding.RoleRef

		return controllerutil.SetControllerReference(k.clusterRoleBinding, rb, tenantClient.Scheme())
	})
}

func (k *KubeProxy) mutateConfigMap(ctx context.Context, tenantClient client.Client) (controllerutil.OperationResult, error) {
	cm := &corev1.ConfigMap{}
	cm.SetName(k.configMap.GetName())
	cm.SetNamespace(k.configMap.GetNamespace())

	return utilities.CreateOrUpdateWithConflict(ctx, tenantClient, cm, func() error {
		cm.SetLabels(utilities.MergeMaps(cm.GetLabels(), k.configMap.GetLabels()))
		cm.SetAnnotations(utilities.MergeMaps(cm.GetAnnotations(), k.configMap.GetAnnotations()))
		cm.Data = k.configMap.Data

		return nil
	})
}

func (k *KubeProxy) mutateDaemonSet(ctx context.Context, tenantClient client.Client) (controllerutil.OperationResult, error) {
	var ds appsv1.DaemonSet
	ds.Name = k.daemonSet.Name
	ds.Namespace = k.daemonSet.Namespace

	if err := tenantClient.Get(ctx, client.ObjectKeyFromObject(&ds), &ds); err != nil {
		if k8serrors.IsNotFound(err) {
			return utilities.CreateOrUpdateWithConflict(ctx, tenantClient, k.daemonSet, func() error {
				return controllerutil.SetControllerReference(k.clusterRoleBinding, k.daemonSet, tenantClient.Scheme())
			})
		}

		return controllerutil.OperationResultNone, err
	}

	if err := controllerutil.SetControllerReference(k.clusterRoleBinding, k.daemonSet, tenantClient.Scheme()); err != nil {
		return controllerutil.OperationResultNone, err
	}

	return controllerutil.OperationResultNone, tenantClient.Patch(ctx, k.daemonSet, client.Apply, client.FieldOwner("kamaji"), client.ForceOwnership)
}

func (k *KubeProxy) decodeManifests(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) error {
	tcpClient, config, err := resources.GetKubeadmManifestDeps(ctx, k.Client, tcp)
	if err != nil {
		return fmt.Errorf("unable to create manifests dependencies: %w", err)
	}
	// If the kube-proxy addon has overrides, adding it to the kubeadm parameters
	config.Parameters.KubeProxyOptions = &kubeadm.AddonOptions{}

	if len(tcp.Spec.Addons.KubeProxy.ImageRepository) > 0 {
		config.Parameters.KubeProxyOptions.Repository = tcp.Spec.Addons.KubeProxy.ImageRepository
	} else {
		config.Parameters.KubeProxyOptions.Repository = "registry.k8s.io"
	}

	if len(tcp.Spec.Addons.KubeProxy.ImageTag) > 0 {
		config.Parameters.KubeProxyOptions.Tag = tcp.Spec.Addons.KubeProxy.ImageTag
	} else {
		config.Parameters.KubeProxyOptions.Tag = tcp.Spec.Kubernetes.Version
	}

	manifests, err := kubeadm.AddKubeProxy(tcpClient, config)
	if err != nil {
		return fmt.Errorf("unable to generate manifests: %w", err)
	}

	parts := bytes.Split(manifests, []byte("---"))

	if err = utilities.DecodeFromYAML(string(parts[1]), k.serviceAccount); err != nil {
		return fmt.Errorf("unable to decode ServiceAccount manifest: %w", err)
	}
	addon_utils.SetKamajiManagedLabels(k.serviceAccount)

	if err = utilities.DecodeFromYAML(string(parts[2]), k.clusterRoleBinding); err != nil {
		return fmt.Errorf("unable to decode ClusterRoleBinding manifest: %w", err)
	}
	addon_utils.SetKamajiManagedLabels(k.clusterRoleBinding)

	if err = utilities.DecodeFromYAML(string(parts[3]), k.role); err != nil {
		return fmt.Errorf("unable to decode Role manifest: %w", err)
	}
	addon_utils.SetKamajiManagedLabels(k.role)

	if err = utilities.DecodeFromYAML(string(parts[4]), k.roleBinding); err != nil {
		return fmt.Errorf("unable to decode RoleBinding manifest: %w", err)
	}
	addon_utils.SetKamajiManagedLabels(k.roleBinding)

	if err = utilities.DecodeFromYAML(string(parts[5]), k.configMap); err != nil {
		return fmt.Errorf("unable to decode ConfigMap manifest: %w", err)
	}
	addon_utils.SetKamajiManagedLabels(k.configMap)

	if err = utilities.DecodeFromYAML(string(parts[6]), k.daemonSet); err != nil {
		return fmt.Errorf("unable to decode DaemonSet manifest: %w", err)
	}
	addon_utils.SetKamajiManagedLabels(k.daemonSet)

	return nil
}
