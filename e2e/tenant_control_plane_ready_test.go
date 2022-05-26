package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("Deploy a TenantControlPlane resource", func() {
	// Fill TenantControlPlane object
	tcp := kamajiv1alpha1.TenantControlPlane{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-test",
			Namespace: "kamaji-system",
		},
		Spec: kamajiv1alpha1.TenantControlPlaneSpec{
			ControlPlane: kamajiv1alpha1.ControlPlane{
				Deployment: kamajiv1alpha1.DeploymentSpec{
					Replicas: 1,
				},
				Ingress: kamajiv1alpha1.IngressSpec{
					Enabled: true,
				},
				Service: kamajiv1alpha1.ServiceSpec{
					ServiceType: "NodePort",
				},
			},
			NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
				Address:       "172.18.0.2",
				DNSServiceIPs: []string{"10.96.0.10"},
				Domain:        "clastix.labs",
				PodCIDR:       "10.244.0.0/16",
				Port:          31443,
				ServiceCIDR:   "10.96.0.0/16",
			},
			Kubernetes: kamajiv1alpha1.KubernetesSpec{
				Version: "v1.23.6",
				Kubelet: kamajiv1alpha1.KubeletSpec{
					CGroupFS: "cgroupfs",
				},
				AdmissionControllers: kamajiv1alpha1.AdmissionControllers{
					"LimitRanger",
					"ResourceQuota",
				},
			},
			Addons: kamajiv1alpha1.AddonsSpec{},
		},
	}

	// Create a TenantControlPlane resource into the cluster
	JustBeforeEach(func() {
		Expect(k8sClient.Create(context.Background(), &tcp)).NotTo(HaveOccurred())
	})

	// Delete the TenantControlPlane resource after test is finished
	JustAfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), &tcp)).Should(Succeed())
	})

	// Check if TenantControlPlane resource has been created
	It("Should be Ready", func() {
		Eventually(func() kamajiv1alpha1.KubernetesVersionStatus {
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      tcp.GetName(),
				Namespace: tcp.GetNamespace(),
			}, &tcp)
			if err != nil {
				return ""
			}

			// Check if Status field has been created on TenantControlPlane struct
			if *&tcp.Status.Kubernetes.Version.Status == nil {
				return ""
			}

			return *tcp.Status.Kubernetes.Version.Status
		}, 5*time.Minute, time.Second).Should(Equal(kamajiv1alpha1.VersionReady))
	})
})
