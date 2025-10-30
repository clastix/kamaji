// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/constants"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/utilities"
)

const (
	AdminKubeConfigFileName             = kubeadmconstants.AdminKubeConfigFileName
	SuperAdminKubeConfigFileName        = kubeadmconstants.SuperAdminKubeConfigFileName
	ControllerManagerKubeConfigFileName = kubeadmconstants.ControllerManagerKubeConfigFileName
	SchedulerKubeConfigFileName         = kubeadmconstants.SchedulerKubeConfigFileName
	localhost                           = "127.0.0.1"
)

type KubeconfigResource struct {
	resource                *corev1.Secret
	Client                  client.Client
	Name                    string
	KubeConfigFileName      string
	TmpDirectory            string
	CertExpirationThreshold time.Duration
}

func (r *KubeconfigResource) GetHistogram() prometheus.Histogram {
	kubeconfigCollector = LazyLoadHistogramFromResource(kubeconfigCollector, r)

	return kubeconfigCollector
}

func (r *KubeconfigResource) ShouldStatusBeUpdated(_ context.Context, tcp *kamajiv1alpha1.TenantControlPlane) bool {
	// an update is required only in case of missing status checksum, or name:
	// this data is required by the following resource handlers.
	status, err := r.getKubeconfigStatus(tcp)
	if err != nil {
		return false
	}

	return len(status.Checksum) == 0 || len(status.SecretName) == 0
}

func (r *KubeconfigResource) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *KubeconfigResource) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *KubeconfigResource) Define(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *KubeconfigResource) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.GetName(), tenantControlPlane)
}

func (r *KubeconfigResource) GetClient() client.Client {
	return r.Client
}

func (r *KubeconfigResource) GetTmpDirectory() string {
	return r.TmpDirectory
}

func (r *KubeconfigResource) GetName() string {
	return r.Name
}

func (r *KubeconfigResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	logger := log.FromContext(ctx, "resource", r.GetName())

	status, err := r.getKubeconfigStatus(tenantControlPlane)
	if err != nil {
		logger.Error(err, "cannot retrieve status")

		return err
	}

	status.LastUpdate = metav1.Now()
	status.SecretName = r.resource.GetName()
	status.Checksum = utilities.GetObjectChecksum(r.resource)

	return nil
}

func (r *KubeconfigResource) getKubeconfigStatus(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*kamajiv1alpha1.KubeconfigStatus, error) {
	switch r.KubeConfigFileName {
	case kubeadmconstants.AdminKubeConfigFileName, kubeadmconstants.SuperAdminKubeConfigFileName:
		return &tenantControlPlane.Status.KubeConfig.Admin, nil
	case kubeadmconstants.ControllerManagerKubeConfigFileName:
		return &tenantControlPlane.Status.KubeConfig.ControllerManager, nil
	case kubeadmconstants.SchedulerKubeConfigFileName:
		return &tenantControlPlane.Status.KubeConfig.Scheduler, nil
	default:
		return nil, fmt.Errorf("kubeconfigfilename %s is not a right name", r.KubeConfigFileName)
	}
}

func (r *KubeconfigResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *KubeconfigResource) checksum(caCertificatesSecret *corev1.Secret, kubeadmChecksum string) string {
	return utilities.CalculateMapChecksum(map[string][]byte{
		"ca-cert-checksum": caCertificatesSecret.Data[kubeadmconstants.CACertName],
		"ca-key-checksum":  caCertificatesSecret.Data[kubeadmconstants.CAKeyName],
		"kubeadmconfig":    []byte(kubeadmChecksum),
	})
}

//nolint:gocognit
func (r *KubeconfigResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		logger := log.FromContext(ctx, "resource", r.GetName())

		config, err := getStoredKubeadmConfiguration(ctx, r.Client, r.TmpDirectory, tenantControlPlane)
		if err != nil {
			logger.Error(err, "cannot retrieve kubeadm configuration")

			return err
		}

		if err = r.customizeConfig(config, tenantControlPlane); err != nil {
			logger.Error(err, "cannot customize the configuration")

			return err
		}

		caSecretNamespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: tenantControlPlane.Status.Certificates.CA.SecretName}
		caCertificatesSecret := &corev1.Secret{}
		if err = r.Client.Get(ctx, caSecretNamespacedName, caCertificatesSecret); err != nil {
			logger.Error(err, "cannot retrieve the CA")

			return err
		}

		checksum := r.checksum(caCertificatesSecret, config.Checksum())

		status, err := r.getKubeconfigStatus(tenantControlPlane)
		if err != nil {
			logger.Error(err, "cannot retrieve status")

			return err
		}

		r.resource.SetLabels(utilities.MergeMaps(
			r.resource.GetLabels(),
			utilities.KamajiLabels(tenantControlPlane.GetName(), r.GetName()),
			map[string]string{
				constants.ControllerLabelResource: utilities.CertificateKubeconfigLabel,
			},
		))
		r.resource.SetAnnotations(utilities.MergeMaps(r.resource.GetAnnotations(), map[string]string{constants.Checksum: checksum}))

		if err = ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme()); err != nil {
			logger.Error(err, "cannot set controller reference", "resource", r.GetName())

			return err
		}

		var shouldCreate bool

		shouldCreate = shouldCreate || r.resource.Data == nil                                            // Missing data key
		shouldCreate = shouldCreate || len(r.resource.Data) == 0                                         // Missing data key
		shouldCreate = shouldCreate || len(r.resource.Data[r.KubeConfigFileName]) == 0                   // Missing kubeconfig file, must be generated
		shouldCreate = shouldCreate || !kubeadm.IsKubeconfigValid(r.resource.Data[r.KubeConfigFileName]) // invalid kubeconfig, or expired client certificate
		shouldCreate = shouldCreate || status.Checksum != checksum || len(r.resource.UID) == 0           // Wrong checksum

		shouldRotate := utilities.IsRotationRequested(r.resource)

		if !shouldCreate {
			v, ok := r.resource.Data[r.KubeConfigFileName]
			shouldCreate = len(v) == 0 || !ok
		}

		if shouldCreate || shouldRotate {
			crtKeyPair := kubeadm.CertificatePrivateKeyPair{
				Certificate: caCertificatesSecret.Data[kubeadmconstants.CACertName],
				PrivateKey:  caCertificatesSecret.Data[kubeadmconstants.CAKeyName],
			}

			if r.resource.Data == nil {
				r.resource.Data = map[string][]byte{}
			}

			kubeconfig, kcErr := kubeadm.CreateKubeconfig(r.KubeConfigFileName, crtKeyPair, config)
			if kcErr != nil {
				logger.Error(kcErr, "cannot create a valid kubeconfig")

				return kcErr
			}

			// Post-process the kubeconfig to use the public API server address for
			// controller manager and scheduler components instead of localhost/IP address
			if strings.Contains(r.KubeConfigFileName, "controller-manager") || strings.Contains(r.KubeConfigFileName, "scheduler") {
				kubeconfig, kcErr = r.replaceServerURLWithPublicAddress(kubeconfig, tenantControlPlane)
				if kcErr != nil {
					logger.Error(kcErr, "cannot replace server URL in kubeconfig")
					return kcErr
				}
			}

			if shouldRotate {
				utilities.SetLastRotationTimestamp(r.resource)
			}

			r.resource.Data[r.KubeConfigFileName] = kubeconfig
			// Adding a kubeconfig useful for the local connections:
			// especially for the admin.conf and super-admin.conf, these would use the public IP address.
			// However, when running in-cluster agents, it would be beneficial having a local connection
			// to avoid unnecessary hops to the LB.
			if strings.Contains(r.KubeConfigFileName, "admin") {
				key := strings.ReplaceAll(r.KubeConfigFileName, ".conf", ".svc")

				config.InitConfiguration.ControlPlaneEndpoint = fmt.Sprintf("%s.%s.svc:%d", tenantControlPlane.Name, tenantControlPlane.Namespace, tenantControlPlane.Spec.NetworkProfile.Port)
				kubeconfig, kcErr = kubeadm.CreateKubeconfig(r.KubeConfigFileName, crtKeyPair, config)
				if kcErr != nil {
					logger.Error(kcErr, "cannot create a valid kubeconfig")

					return kcErr
				}

				r.resource.Data[key] = kubeconfig
			}
		}

		return nil
	}
}

func (r *KubeconfigResource) customizeConfig(config *kubeadm.Configuration, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	switch r.KubeConfigFileName {
	case kubeadmconstants.ControllerManagerKubeConfigFileName:
		return r.usePublicAPIServerAddress(config, tenantControlPlane)
	case kubeadmconstants.SchedulerKubeConfigFileName:
		return r.usePublicAPIServerAddress(config, tenantControlPlane)
	default:
		return nil
	}
}

func (r *KubeconfigResource) usePublicAPIServerAddress(config *kubeadm.Configuration, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	// Use the public API server endpoint configured in the TenantControlPlane
	// instead of localhost for controller manager and scheduler kubeconfigs.
	// This ensures that internal control plane components connect using the proper hostname
	// which matches the certificate SANs, rather than failing with x509 certificate errors.

	// Note: We don't modify the kubeadm configuration here because LocalAPIEndpoint.AdvertiseAddress
	// must be an IP address according to kubeadm validation. Instead, we'll post-process
	// the generated kubeconfig to replace the server URL with the hostname.

	return nil
}

func (r *KubeconfigResource) replaceServerURLWithPublicAddress(kubeconfigBytes []byte, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) ([]byte, error) {
	// Parse the kubeconfig YAML
	var kubeconfig map[string]interface{}
	if err := yaml.Unmarshal(kubeconfigBytes, &kubeconfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal kubeconfig: %w", err)
	}

	// Get the public API server address from the TenantControlPlane
	publicHost, publicPort, err := tenantControlPlane.PublicControlPlaneAddress()
	if err != nil {
		return nil, fmt.Errorf("failed to get public control plane address: %w", err)
	}
	if publicHost == "" {
		// If no public address is configured, return the original kubeconfig unchanged
		return kubeconfigBytes, nil
	}

	// Construct the full public API server URL
	// Check if publicHost already contains a scheme (full URL)
	var publicAddress string
	if strings.HasPrefix(publicHost, "https://") || strings.HasPrefix(publicHost, "http://") {
		// publicHost is already a full URL, use it as-is
		publicAddress = publicHost
	} else {
		// publicHost is just a hostname, construct the full URL
		publicAddress = fmt.Sprintf("https://%s:%d", publicHost, publicPort)
	}

	// Navigate to clusters[0].cluster.server and replace it
	clusters, ok := kubeconfig["clusters"].([]interface{})
	if !ok || len(clusters) == 0 {
		return nil, fmt.Errorf("kubeconfig has no clusters")
	}

	cluster, ok := clusters[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid cluster format")
	}

	clusterData, ok := cluster["cluster"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid cluster data format")
	}

	// Replace the server URL with the public address
	clusterData["server"] = publicAddress

	// Marshal back to YAML
	modifiedKubeconfig, err := yaml.Marshal(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified kubeconfig: %w", err)
	}

	return modifiedKubeconfig, nil
}
