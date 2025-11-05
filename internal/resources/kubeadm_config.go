// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/utilities"
)

type KubeadmConfigResource struct {
	resource     *corev1.ConfigMap
	Client       client.Client
	ETCDs        []string
	TmpDirectory string
}

func (r *KubeadmConfigResource) GetHistogram() prometheus.Histogram {
	kubeadmconfigCollector = LazyLoadHistogramFromResource(kubeadmconfigCollector, r)

	return kubeadmconfigCollector
}

func (r *KubeadmConfigResource) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return tenantControlPlane.Status.KubeadmConfig.Checksum != utilities.GetObjectChecksum(r.resource)
}

func (r *KubeadmConfigResource) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *KubeadmConfigResource) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *KubeadmConfigResource) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *KubeadmConfigResource) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.GetName(), tenantControlPlane)
}

func (r *KubeadmConfigResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *KubeadmConfigResource) GetName() string {
	return "kubeadmconfig"
}

func (r *KubeadmConfigResource) UpdateTenantControlPlaneStatus(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	tenantControlPlane.Status.KubeadmConfig.LastUpdate = metav1.Now()
	tenantControlPlane.Status.KubeadmConfig.Checksum = utilities.GetObjectChecksum(r.resource)
	tenantControlPlane.Status.KubeadmConfig.ConfigmapName = r.resource.GetName()

	return nil
}

func (r *KubeadmConfigResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())

		address, port, err := tenantControlPlane.AssignedControlPlaneAddress()
		if err != nil {
			logger.Error(err, "cannot retrieve Tenant Control Plane address")

			return err
		}

		r.resource.SetLabels(utilities.MergeMaps(r.resource.GetLabels(), utilities.KamajiLabels(tenantControlPlane.GetName(), r.GetName())))

		// TODO: This logic should be part of the status update logic of the
		// tenant I think. Ie, tenant should tell the URL.
		endpoint := net.JoinHostPort(address, strconv.FormatInt(int64(port), 10))
		ks := tenantControlPlane.Status.Kubernetes
		if (ks.Gateway != nil) && (ks.Ingress != nil) {
			return fmt.Errorf("using both gateway and ingress is not supported")
		}
		if ks.Gateway != nil {
			if len(ks.Gateway.AccessPoints) != 1 {
				return fmt.Errorf("gateway has more than one endpoint")
			}
			rawUrl := ks.Gateway.AccessPoints[0].Value
			parsedURL, err := url.Parse(rawUrl)
			if err != nil {
				return fmt.Errorf("invalid endpoint '%s': %w ", rawUrl, err)
			}
			endpoint = net.JoinHostPort(parsedURL.Hostname(), parsedURL.Port())
		}
		if ks.Ingress != nil {
			ingress := tenantControlPlane.Spec.ControlPlane.Ingress
			if ingress != nil && len(ingress.Hostname) > 0 {
				iaddr, iport := utilities.GetControlPlaneAddressAndPortFromHostname(ingress.Hostname, port)
				endpoint = net.JoinHostPort(iaddr, strconv.FormatInt(int64(iport), 10))
			}
		}

		params := kubeadm.Parameters{
			TenantControlPlaneAddress:       address,
			TenantControlPlanePort:          port,
			TenantControlPlaneName:          tenantControlPlane.GetName(),
			TenantControlPlaneNamespace:     tenantControlPlane.GetNamespace(),
			TenantControlPlaneEndpoint:      endpoint,
			TenantControlPlaneCertSANs:      tenantControlPlane.Spec.NetworkProfile.CertSANs,
			TenantControlPlaneClusterDomain: tenantControlPlane.Spec.NetworkProfile.ClusterDomain,
			TenantControlPlanePodCIDR:       tenantControlPlane.Spec.NetworkProfile.PodCIDR,
			TenantControlPlaneServiceCIDR:   tenantControlPlane.Spec.NetworkProfile.ServiceCIDR,
			TenantControlPlaneVersion:       tenantControlPlane.Spec.Kubernetes.Version,
			ETCDs:                           r.ETCDs,
			CertificatesDir:                 r.TmpDirectory,
		}

		config, err := kubeadm.CreateKubeadmInitConfiguration(params)
		if err != nil {
			return err
		}
		if r.resource.Data, err = kubeadm.GetKubeadmInitConfigurationMap(*config); err != nil {
			logger.Error(err, "cannot retrieve kubeadm init configuration")

			return err
		}

		utilities.SetObjectChecksum(r.resource, r.resource.Data)

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}
