// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/resources"
)

var _ = Describe("APIServerCertificate CreateOrUpdate", func() {
	const tcpName = "test-tcp"

	It("regenerates the certificate when the stored Secret has no certificate entry", func() {
		ctx := context.Background()
		tmpDirectory := GinkgoT().TempDir()

		config, err := kubeadm.CreateKubeadmInitConfiguration(kubeadm.Parameters{
			TenantControlPlaneName:          tcpName,
			TenantControlPlaneNamespace:     "default",
			TenantControlPlaneAddress:       "192.0.2.1",
			TenantControlPlanePort:          6443,
			TenantControlPlaneClusterDomain: "cluster.local",
			TenantControlPlanePodCIDR:       []string{"10.244.0.0/16"},
			TenantControlPlaneServiceCIDR:   []string{"10.96.0.0/12"},
			TenantControlPlaneVersion:       "v1.36.1",
			ETCDs:                           []string{"http://etcd.default.svc:2379"},
		})
		Expect(err).ToNot(HaveOccurred())
		config.InitConfiguration.CertificatesDir = tmpDirectory

		ca, err := kubeadm.GenerateCACertificatePrivateKeyPair(kubeadmconstants.CACertAndKeyBaseName, config)
		Expect(err).ToNot(HaveOccurred())

		configData, err := kubeadm.GetKubeadmInitConfigurationMap(*config)
		Expect(err).ToNot(HaveOccurred())

		tcp := &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: tcpName, Namespace: "default", UID: "tcp-uid"},
			Status: kamajiv1alpha1.TenantControlPlaneStatus{
				KubeadmConfig: kamajiv1alpha1.KubeadmConfigStatus{ConfigmapName: tcpName + "-kubeadmconfig"},
				Certificates: kamajiv1alpha1.CertificatesStatus{
					CA: kamajiv1alpha1.CertificatePrivateKeyPairStatus{SecretName: tcpName + "-ca"},
				},
			},
		}
		// The Secret already exists, but its certificate entry is gone.
		existing := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: tcpName + "-api-server-certificate", Namespace: "default", UID: "secret-uid"},
			Data:       map[string][]byte{kubeadmconstants.APIServerKeyName: []byte("stale")},
		}
		fakeClient := fake.NewClientBuilder().
			WithScheme(runtimeScheme).
			WithObjects(
				existing,
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: tcpName + "-kubeadmconfig", Namespace: "default"}, Data: configData},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: tcpName + "-ca", Namespace: "default"},
					Data:       map[string][]byte{kubeadmconstants.CACertName: ca.Certificate, kubeadmconstants.CAKeyName: ca.PrivateKey},
				},
			).
			Build()
		resource := &resources.APIServerCertificate{Client: fakeClient, TmpDirectory: tmpDirectory}
		Expect(resource.Define(ctx, tcp)).To(Succeed())

		result, err := resource.CreateOrUpdate(ctx, tcp)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(controllerutil.OperationResultUpdated))

		var secret corev1.Secret
		Expect(fakeClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: existing.Name}, &secret)).To(Succeed())
		Expect(secret.Data[kubeadmconstants.APIServerCertName]).ToNot(BeEmpty())
	})
})
