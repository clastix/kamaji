// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Kubelet preferredAddressTypes", func() {
	var (
		ctx context.Context
		tcp *TenantControlPlane
	)

	BeforeEach(func() {
		ctx = context.Background()
		tcp = &TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tcp-preferred-address-types",
				Namespace: "default",
			},
			Spec: TenantControlPlaneSpec{},
		}
		tcp.Spec.ControlPlane.Service.ServiceType = ServiceTypeClusterIP
	})

	AfterEach(func() {
		if err := k8sClient.Delete(ctx, tcp); err != nil && !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("defaults to InternalIP, ExternalIP, Hostname in that order", func() {
		Expect(k8sClient.Create(ctx, tcp)).ToNot(HaveOccurred())

		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tcp.Name, Namespace: tcp.Namespace}, tcp)).ToNot(HaveOccurred())
		Expect(tcp.Spec.Kubernetes.Kubelet.PreferredAddressTypes).To(Equal([]KubeletPreferredAddressType{
			NodeInternalIP, NodeExternalIP, NodeHostName,
		}))
	})

	It("stores a non-default order verbatim", func() {
		// The API server takes the first address type a node reports, so the order
		// is part of the meaning of this field and must survive a round-trip.
		tcp.Spec.Kubernetes.Kubelet.PreferredAddressTypes = []KubeletPreferredAddressType{
			NodeHostName, NodeInternalIP,
		}

		Expect(k8sClient.Create(ctx, tcp)).ToNot(HaveOccurred())

		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tcp.Name, Namespace: tcp.Namespace}, tcp)).ToNot(HaveOccurred())
		Expect(tcp.Spec.Kubernetes.Kubelet.PreferredAddressTypes).To(Equal([]KubeletPreferredAddressType{
			NodeHostName, NodeInternalIP,
		}))
	})

	It("keeps the applied order when a second field manager owns entries too", func() {
		// listType=set declares the entries unordered, and structured-merge-diff then
		// interleaves the two sides rather than taking the applier's list: a second
		// manager applying [Hostname, InternalDNS] used to end up with
		// [InternalIP, ExternalIP, Hostname, InternalDNS] stored. Since the API server
		// takes the first address type a node reports, that silently changes which
		// address kubelet connections are attempted on.
		apply := func(manager string, addressTypes ...KubeletPreferredAddressType) {
			obj := &TenantControlPlane{
				TypeMeta:   metav1.TypeMeta{APIVersion: GroupVersion.String(), Kind: "TenantControlPlane"},
				ObjectMeta: metav1.ObjectMeta{Name: tcp.Name, Namespace: tcp.Namespace},
			}
			obj.Spec.ControlPlane.Service.ServiceType = ServiceTypeClusterIP
			obj.Spec.Kubernetes.Kubelet.PreferredAddressTypes = addressTypes

			Expect(k8sClient.Patch(ctx, obj, client.Apply, client.FieldOwner(manager), client.ForceOwnership)).ToNot(HaveOccurred())
		}

		apply("manager-a", NodeInternalIP, NodeExternalIP, NodeHostName)
		apply("manager-b", NodeHostName, NodeInternalDNS)

		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: tcp.Name, Namespace: tcp.Namespace}, tcp)).ToNot(HaveOccurred())
		Expect(tcp.Spec.Kubernetes.Kubelet.PreferredAddressTypes).To(Equal([]KubeletPreferredAddressType{
			NodeHostName, NodeInternalDNS,
		}))
	})

	It("rejects duplicated entries", func() {
		tcp.Spec.Kubernetes.Kubelet.PreferredAddressTypes = []KubeletPreferredAddressType{
			NodeInternalIP, NodeExternalIP, NodeInternalIP,
		}

		err := k8sClient.Create(ctx, tcp)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("preferredAddressTypes entries must be unique"))
	})
})
