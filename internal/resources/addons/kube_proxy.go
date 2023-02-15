// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"bytes"
	"context"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/resources"
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
	return tenantControlPlane.Spec.Addons.KubeProxy == nil
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
		if err = tenantClient.Delete(ctx, obj); err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}

			return false, err
		}
		deleted = deleted || err == nil
	}

	return deleted, nil
}

func (k *KubeProxy) CreateOrUpdate(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
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
		crb.SetLabels(k.clusterRoleBinding.GetLabels())
		crb.SetAnnotations(k.clusterRoleBinding.GetAnnotations())
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
		sa.SetLabels(k.serviceAccount.GetLabels())
		sa.SetAnnotations(k.serviceAccount.GetAnnotations())

		return controllerutil.SetControllerReference(k.clusterRoleBinding, sa, tenantClient.Scheme())
	})
}

func (k *KubeProxy) mutateRole(ctx context.Context, tenantClient client.Client) (controllerutil.OperationResult, error) {
	r := &rbacv1.Role{}
	r.SetName(k.role.GetName())
	r.SetNamespace(k.role.GetNamespace())

	return utilities.CreateOrUpdateWithConflict(ctx, tenantClient, r, func() error {
		r.SetLabels(k.role.GetLabels())
		r.SetAnnotations(k.role.GetAnnotations())
		r.Rules = k.role.Rules

		return controllerutil.SetControllerReference(k.clusterRoleBinding, r, tenantClient.Scheme())
	})
}

func (k *KubeProxy) mutateRoleBinding(ctx context.Context, tenantClient client.Client) (controllerutil.OperationResult, error) {
	rb := &rbacv1.RoleBinding{}
	rb.SetName(k.roleBinding.GetName())
	rb.SetNamespace(k.roleBinding.GetNamespace())

	return utilities.CreateOrUpdateWithConflict(ctx, tenantClient, rb, func() error {
		rb.SetLabels(k.roleBinding.GetLabels())
		rb.SetAnnotations(k.roleBinding.GetAnnotations())
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
		cm.SetLabels(k.configMap.GetLabels())
		cm.SetAnnotations(k.configMap.GetAnnotations())
		cm.Data = k.configMap.Data

		return nil
	})
}

func (k *KubeProxy) mutateDaemonSet(ctx context.Context, tenantClient client.Client) (controllerutil.OperationResult, error) {
	ds := &appsv1.DaemonSet{}
	ds.SetName(k.daemonSet.GetName())
	ds.SetNamespace(k.daemonSet.GetNamespace())

	return utilities.CreateOrUpdateWithConflict(ctx, tenantClient, ds, func() error {
		ds.SetLabels(k.daemonSet.GetLabels())
		ds.SetAnnotations(utilities.MergeMaps(ds.GetAnnotations(), k.daemonSet.GetAnnotations()))
		ds.Spec.Selector = k.daemonSet.Spec.Selector
		if len(ds.Spec.Template.Spec.Volumes) != 3 {
			ds.Spec.Template.Spec.Volumes = make([]corev1.Volume, 3)
		}
		ds.Spec.Template.ObjectMeta.SetLabels(k.daemonSet.Spec.Template.GetLabels())
		ds.Spec.Template.Spec.Volumes[0].Name = k.daemonSet.Spec.Template.Spec.Volumes[0].Name
		ds.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap = &corev1.ConfigMapVolumeSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: k.daemonSet.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap.Name},
			DefaultMode:          pointer.Int32(420),
		}

		ds.Spec.Template.Spec.Volumes[1].Name = k.daemonSet.Spec.Template.Spec.Volumes[1].Name
		ds.Spec.Template.Spec.Volumes[1].VolumeSource.HostPath = &corev1.HostPathVolumeSource{
			Path: k.daemonSet.Spec.Template.Spec.Volumes[1].VolumeSource.HostPath.Path,
			Type: func(v corev1.HostPathType) *corev1.HostPathType {
				return &v
			}(corev1.HostPathFileOrCreate),
		}

		ds.Spec.Template.Spec.Volumes[2].Name = k.daemonSet.Spec.Template.Spec.Volumes[2].Name
		ds.Spec.Template.Spec.Volumes[2].VolumeSource.HostPath = &corev1.HostPathVolumeSource{
			Path: k.daemonSet.Spec.Template.Spec.Volumes[2].VolumeSource.HostPath.Path,
			Type: func(v corev1.HostPathType) *corev1.HostPathType {
				return &v
			}(""),
		}

		if len(ds.Spec.Template.Spec.Containers) == 0 {
			ds.Spec.Template.Spec.Containers = make([]corev1.Container, 1)
		}
		ds.Spec.Template.Spec.Containers[0].Name = k.daemonSet.Spec.Template.Spec.Containers[0].Name
		ds.Spec.Template.Spec.Containers[0].Image = k.daemonSet.Spec.Template.Spec.Containers[0].Image
		ds.Spec.Template.Spec.Containers[0].Command = k.daemonSet.Spec.Template.Spec.Containers[0].Command
		if len(ds.Spec.Template.Spec.Containers[0].Env) == 0 {
			ds.Spec.Template.Spec.Containers[0].Env = make([]corev1.EnvVar, 1)
		}
		ds.Spec.Template.Spec.Containers[0].Env[0].Name = k.daemonSet.Spec.Template.Spec.Containers[0].Env[0].Name
		if ds.Spec.Template.Spec.Containers[0].Env[0].ValueFrom == nil {
			ds.Spec.Template.Spec.Containers[0].Env[0].ValueFrom = &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{},
			}
		}
		ds.Spec.Template.Spec.Containers[0].Env[0].ValueFrom.FieldRef.FieldPath = k.daemonSet.Spec.Template.Spec.Containers[0].Env[0].ValueFrom.FieldRef.FieldPath
		if len(ds.Spec.Template.Spec.Containers[0].VolumeMounts) != 3 {
			ds.Spec.Template.Spec.Containers[0].VolumeMounts = make([]corev1.VolumeMount, 3)
		}
		ds.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name = k.daemonSet.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name
		ds.Spec.Template.Spec.Containers[0].VolumeMounts[0].ReadOnly = k.daemonSet.Spec.Template.Spec.Containers[0].VolumeMounts[0].ReadOnly
		ds.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath = k.daemonSet.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath
		ds.Spec.Template.Spec.Containers[0].VolumeMounts[1].Name = k.daemonSet.Spec.Template.Spec.Containers[0].VolumeMounts[1].Name
		ds.Spec.Template.Spec.Containers[0].VolumeMounts[1].ReadOnly = k.daemonSet.Spec.Template.Spec.Containers[0].VolumeMounts[1].ReadOnly
		ds.Spec.Template.Spec.Containers[0].VolumeMounts[1].MountPath = k.daemonSet.Spec.Template.Spec.Containers[0].VolumeMounts[1].MountPath
		ds.Spec.Template.Spec.Containers[0].VolumeMounts[2].Name = k.daemonSet.Spec.Template.Spec.Containers[0].VolumeMounts[2].Name
		ds.Spec.Template.Spec.Containers[0].VolumeMounts[2].ReadOnly = k.daemonSet.Spec.Template.Spec.Containers[0].VolumeMounts[2].ReadOnly
		ds.Spec.Template.Spec.Containers[0].VolumeMounts[2].MountPath = k.daemonSet.Spec.Template.Spec.Containers[0].VolumeMounts[2].MountPath
		if ds.Spec.Template.Spec.Containers[0].SecurityContext == nil {
			ds.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{}
		}
		ds.Spec.Template.Spec.Containers[0].SecurityContext.Privileged = k.daemonSet.Spec.Template.Spec.Containers[0].SecurityContext.Privileged
		ds.Spec.Template.Spec.NodeSelector = k.daemonSet.Spec.Template.Spec.NodeSelector
		ds.Spec.Template.Spec.ServiceAccountName = k.daemonSet.Spec.Template.Spec.ServiceAccountName
		ds.Spec.Template.Spec.HostNetwork = k.daemonSet.Spec.Template.Spec.HostNetwork
		ds.Spec.Template.Spec.Tolerations = k.daemonSet.Spec.Template.Spec.Tolerations
		ds.Spec.Template.Spec.PriorityClassName = k.daemonSet.Spec.Template.Spec.PriorityClassName
		ds.Spec.UpdateStrategy.Type = k.daemonSet.Spec.UpdateStrategy.Type

		return controllerutil.SetControllerReference(k.clusterRoleBinding, ds, tenantClient.Scheme())
	})
}

func (k *KubeProxy) decodeManifests(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) error {
	tcpClient, config, err := resources.GetKubeadmManifestDeps(ctx, k.Client, tcp)
	if err != nil {
		return errors.Wrap(err, "unable to create manifests dependencies")
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
		return errors.Wrap(err, "unable to generate manifests")
	}

	parts := bytes.Split(manifests, []byte("---"))

	if err = utilities.DecodeFromYAML(string(parts[1]), k.serviceAccount); err != nil {
		return errors.Wrap(err, "unable to decode ServiceAccount manifest")
	}

	if err = utilities.DecodeFromYAML(string(parts[2]), k.clusterRoleBinding); err != nil {
		return errors.Wrap(err, "unable to decode ClusterRoleBinding manifest")
	}

	if err = utilities.DecodeFromYAML(string(parts[3]), k.role); err != nil {
		return errors.Wrap(err, "unable to decode Role manifest")
	}

	if err = utilities.DecodeFromYAML(string(parts[4]), k.roleBinding); err != nil {
		return errors.Wrap(err, "unable to decode RoleBinding manifest")
	}

	if err = utilities.DecodeFromYAML(string(parts[5]), k.configMap); err != nil {
		return errors.Wrap(err, "unable to decode ConfigMap manifest")
	}

	if err = utilities.DecodeFromYAML(string(parts[6]), k.daemonSet); err != nil {
		return errors.Wrap(err, "unable to decode DaemonSet manifest")
	}

	return nil
}
