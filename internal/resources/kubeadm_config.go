// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"net"
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

// canonicalSAN normalises an IP SAN to its canonical net.IP string form so that
// the same address in different textual forms (notably IPv6) collapses to one key;
// non-IP entries (DNS names) pass through unchanged.
func canonicalSAN(s string) string {
	if ip := net.ParseIP(s); ip != nil {
		return ip.String()
	}

	return s
}

// mergeCertSANs appends the additional IPs (typically the control-plane Service IPs)
// to the existing cert SANs, skipping any whose canonical IP form already appears as
// the management address or among the existing SANs. This keeps a single-stack Service
// a no-op (its only IP equals the management address) and prevents duplicate SANs that
// would otherwise churn the certificate.
func mergeCertSANs(managementAddress string, certSANs, additional []string) []string {
	seen := map[string]struct{}{canonicalSAN(managementAddress): {}}
	for _, san := range certSANs {
		seen[canonicalSAN(san)] = struct{}{}
	}

	for _, ip := range additional {
		key := canonicalSAN(ip)
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		certSANs = append(certSANs, ip)
	}

	return certSANs
}

func (r *KubeadmConfigResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())

		address, port, err := tenantControlPlane.AssignedControlPlaneAddress()
		if err != nil {
			logger.Error(err, "cannot retrieve Tenant Control Plane address")

			return err
		}

		// Resolve the advertised address for tenant-side consumers
		advAddress, _, advErr := tenantControlPlane.AdvertisedControlPlaneAddress()
		if advErr != nil {
			return advErr
		}

		r.resource.SetLabels(utilities.MergeMaps(r.resource.GetLabels(), utilities.KamajiLabels(tenantControlPlane.GetName(), r.GetName())))

		endpoint := net.JoinHostPort(advAddress, strconv.FormatInt(int64(port), 10))

		spec := tenantControlPlane.Spec.ControlPlane
		if spec.Gateway != nil {
			if len(spec.Gateway.Hostname) > 0 {
				gaddr, gport := utilities.GetControlPlaneAddressAndPortFromHostname(string(spec.Gateway.Hostname), port)
				endpoint = net.JoinHostPort(gaddr, strconv.FormatInt(int64(gport), 10))
			}
		}

		if spec.Ingress != nil {
			if len(spec.Ingress.Hostname) > 0 {
				iaddr, iport := utilities.GetControlPlaneAddressAndPortFromHostname(spec.Ingress.Hostname, port)
				endpoint = net.JoinHostPort(iaddr, strconv.FormatInt(int64(iport), 10))
			}
		}

		// Add advertise address to cert SANs if different from management address.
		// Copy the spec slice so appends below never mutate the user's CertSANs backing array.
		certSANs := append([]string{}, tenantControlPlane.Spec.NetworkProfile.CertSANs...)
		if advAddress != address {
			certSANs = append(certSANs, advAddress)
		}

		// Add every IP the control-plane Service answers on (all ClusterIPs of a
		// dual-stack Service and any LoadBalancer ingress IPs) so the API server
		// certificate is valid over both families. Deduplicate against the management
		// address and the SANs already present so a single-stack Service adds nothing
		// and the certificate does not churn.
		serviceIPs, svcErr := tenantControlPlane.ControlPlaneServiceIPs(ctx, r.Client)
		if svcErr != nil {
			logger.Error(svcErr, "cannot retrieve control plane Service IPs for cert SANs")

			return svcErr
		}

		certSANs = mergeCertSANs(address, certSANs, serviceIPs)

		// Backward compatibility for deprecated CIDR fields
		if tenantControlPlane.Spec.NetworkProfile.ServiceCIDR != "" {
			logger.Info("serviceCidr is deprecated, please migrate to serviceCidrs")
		}

		if tenantControlPlane.Spec.NetworkProfile.PodCIDR != "" {
			logger.Info("podCidr is deprecated, please migrate to podCidrs")
		}

		serviceCIDRs := utilities.GetEffectiveCIDRs(tenantControlPlane.Spec.NetworkProfile.ServiceCIDR, tenantControlPlane.Spec.NetworkProfile.ServiceCIDRs)
		podCIDRs := utilities.GetEffectiveCIDRs(tenantControlPlane.Spec.NetworkProfile.PodCIDR, tenantControlPlane.Spec.NetworkProfile.PodCIDRs)

		params := kubeadm.Parameters{
			TenantControlPlaneAddress:       address,
			TenantControlPlanePort:          port,
			TenantControlPlaneName:          tenantControlPlane.GetName(),
			TenantControlPlaneNamespace:     tenantControlPlane.GetNamespace(),
			TenantControlPlaneEndpoint:      endpoint,
			TenantControlPlaneCertSANs:      certSANs,
			TenantControlPlaneClusterDomain: tenantControlPlane.Spec.NetworkProfile.ClusterDomain,
			TenantControlPlanePodCIDR:       podCIDRs,
			TenantControlPlaneServiceCIDR:   serviceCIDRs,
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
