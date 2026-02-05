// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegaTypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	pointer "k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

func GetKindIPAddress() string {
	var ep discoveryv1.EndpointSlice
	Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: "kubernetes", Namespace: "default"}, &ep)).ToNot(HaveOccurred())

	return ep.Endpoints[0].Addresses[0]
}

func CreateKindTCPWithAddons(tcpNamespace string, tcpName string, addons kamajiv1alpha1.AddonsSpec) *kamajiv1alpha1.TenantControlPlane {
	return &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tcpName,
			Namespace: tcpNamespace,
		},
		Spec: kamajiv1alpha1.TenantControlPlaneSpec{
			ControlPlane: kamajiv1alpha1.ControlPlane{
				Deployment: kamajiv1alpha1.DeploymentSpec{
					Replicas: pointer.To(int32(1)),
				},
				Service: kamajiv1alpha1.ServiceSpec{
					ServiceType: "NodePort",
				},
			},
			NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
				Address:                  GetKindIPAddress(),
				AllowAddressAsExternalIP: true,
				Port:                     30001,
			},
			Kubernetes: kamajiv1alpha1.KubernetesSpec{
				Version: "v1.28.0",
				Kubelet: kamajiv1alpha1.KubeletSpec{
					CGroupFS: "cgroupfs",
				},
				AdmissionControllers: kamajiv1alpha1.AdmissionControllers{
					"LimitRanger",
					"ResourceQuota",
				},
			},
			Addons: addons,
		},
	}
}

func PrintTenantControlPlaneInfo() {
	tcpList := &kamajiv1alpha1.TenantControlPlaneList{}
	Expect(k8sClient.List(context.Background(), tcpList)).ToNot(HaveOccurred())

	if len(tcpList.Items) == 0 {
		return
	}

	tcp := tcpList.Items[0]

	kubectlExec := func(args ...string) {
		cmd := exec.Command("kubectl")

		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Args = args

		Expect(cmd.Run()).ToNot(HaveOccurred())

		for {
			line, err := out.ReadString('\n')
			if err != nil {
				return
			}

			_, _ = fmt.Fprint(GinkgoWriter, ">>> ", line)
		}
	}

	if CurrentSpecReport().Failed() {
		_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: Tenant Control Plane definition")
		kubectlExec(
			fmt.Sprintf("--namespace=%s", tcp.GetNamespace()),
			"get",
			"tcp",
			tcp.GetName(),
		)
		_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: Tenant Control Plane resources")
		kubectlExec(
			fmt.Sprintf("--namespace=%s", tcp.GetNamespace()),
			"get",
			"svc,deployment,pods,ep,configmap,secrets",
		)
		_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: Tenant Control Plane pods")
		kubectlExec(
			fmt.Sprintf("--namespace=%s", tcp.GetNamespace()),
			"describe",
			"pods",
		)
	}
}

func PrintKamajiLogs() {
	if CurrentSpecReport().Failed() {
		clientset, err := kubernetes.NewForConfig(cfg)
		Expect(err).ToNot(HaveOccurred())

		list, err := clientset.CoreV1().Pods("kamaji-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/component=controller-manager",
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(list.Items).To(HaveLen(1))

		request := clientset.CoreV1().Pods("kamaji-system").GetLogs(list.Items[0].GetName(), &corev1.PodLogOptions{
			Container: "manager",
			SinceSeconds: func() *int64 {
				seconds := int64(CurrentSpecReport().RunTime)

				return &seconds
			}(),
			Timestamps: true,
		})

		podLogs, err := request.Stream(context.Background())
		Expect(err).ToNot(HaveOccurred())

		defer podLogs.Close()

		podBytes, err := io.ReadAll(podLogs)
		Expect(err).ToNot(HaveOccurred())

		_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: retrieving Kamaji Pod logs")

		for _, line := range bytes.Split(podBytes, []byte("\n")) {
			_, _ = fmt.Fprintln(GinkgoWriter, ">>> ", string(line))
		}

		_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: end of Kamaji Pod logs")
	}
}

func StatusMustEqualTo(tcp *kamajiv1alpha1.TenantControlPlane, status kamajiv1alpha1.KubernetesVersionStatus) {
	GinkgoHelper()
	Eventually(func() kamajiv1alpha1.KubernetesVersionStatus {
		err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tcp), tcp)
		if err != nil {
			return ""
		}
		// Check if Status field has been created on TenantControlPlane struct
		if tcp.Status.Kubernetes.Version.Status == nil {
			return ""
		}

		return *tcp.Status.Kubernetes.Version.Status
	}, 5*time.Minute, time.Second).Should(Equal(status))
}

func AllPodsLabelMustEqualTo(tcp *kamajiv1alpha1.TenantControlPlane, label string, value string) {
	GinkgoHelper()
	Eventually(func() bool {
		tcpPods := &corev1.PodList{}
		err := k8sClient.List(context.Background(), tcpPods, client.MatchingLabels{
			"kamaji.clastix.io/name": tcp.GetName(),
		})
		if err != nil {
			return false
		}
		for _, pod := range tcpPods.Items {
			if pod.Labels[label] != value {
				return false
			}
		}

		return true
	}, 5*time.Minute, time.Second).Should(BeTrue())
}

func AllPodsAnnotationMustEqualTo(tcp *kamajiv1alpha1.TenantControlPlane, annotation string, value string) {
	GinkgoHelper()
	Eventually(func() bool {
		tcpPods := &corev1.PodList{}
		err := k8sClient.List(context.Background(), tcpPods, client.MatchingLabels{
			"kamaji.clastix.io/name": tcp.GetName(),
		})
		if err != nil {
			return false
		}
		for _, pod := range tcpPods.Items {
			if pod.Annotations[annotation] != value {
				return false
			}
		}

		return true
	}, 5*time.Minute, time.Second).Should(BeTrue())
}

func PodsServiceAccountMustEqualTo(tcp *kamajiv1alpha1.TenantControlPlane, sa *corev1.ServiceAccount) {
	GinkgoHelper()
	saName := sa.GetName()
	Eventually(func() bool {
		tcpPods := &corev1.PodList{}
		err := k8sClient.List(context.Background(), tcpPods, client.MatchingLabels{
			"kamaji.clastix.io/name": tcp.GetName(),
		})
		if err != nil {
			return false
		}
		for _, pod := range tcpPods.Items {
			if pod.Spec.ServiceAccountName != saName {
				return false
			}
		}

		return true
	}, 5*time.Minute, time.Second).Should(BeTrue())
}

func ScaleTenantControlPlane(tcp *kamajiv1alpha1.TenantControlPlane, replicas int32) {
	GinkgoHelper()
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tcp), tcp)).To(Succeed())
		tcp.Spec.ControlPlane.Deployment.Replicas = &replicas

		return k8sClient.Update(context.Background(), tcp)
	})
	Expect(err).To(Succeed())
}

// CreateGatewayWithListeners creates a Gateway with both kube-apiserver and konnectivity-server listeners.
func CreateGatewayWithListeners(gatewayName, namespace, gatewayClassName, hostname string) {
	GinkgoHelper()
	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gatewayName,
			Namespace: namespace,
		},
		Spec: gatewayv1.GatewaySpec{
			GatewayClassName: gatewayv1.ObjectName(gatewayClassName),
			Listeners: []gatewayv1.Listener{
				{
					Name:     "kube-apiserver",
					Port:     6443,
					Protocol: gatewayv1.TLSProtocolType,
					Hostname: pointer.To(gatewayv1.Hostname(hostname)),
					TLS: &gatewayv1.ListenerTLSConfig{
						Mode: pointer.To(gatewayv1.TLSModeType("Passthrough")),
					},
					AllowedRoutes: &gatewayv1.AllowedRoutes{
						Namespaces: &gatewayv1.RouteNamespaces{
							From: pointer.To(gatewayv1.NamespacesFromAll),
						},
						Kinds: []gatewayv1.RouteGroupKind{
							{
								Group: pointer.To(gatewayv1.Group("gateway.networking.k8s.io")),
								Kind:  "TLSRoute",
							},
						},
					},
				},
				{
					Name:     "konnectivity-server",
					Port:     8132,
					Protocol: gatewayv1.TLSProtocolType,
					Hostname: pointer.To(gatewayv1.Hostname(hostname)),
					TLS: &gatewayv1.ListenerTLSConfig{
						Mode: pointer.To(gatewayv1.TLSModeType("Passthrough")),
					},
					AllowedRoutes: &gatewayv1.AllowedRoutes{
						Namespaces: &gatewayv1.RouteNamespaces{
							From: pointer.To(gatewayv1.NamespacesFromAll),
						},
						Kinds: []gatewayv1.RouteGroupKind{
							{
								Group: pointer.To(gatewayv1.Group("gateway.networking.k8s.io")),
								Kind:  "TLSRoute",
							},
						},
					},
				},
			},
		},
	}
	Expect(k8sClient.Create(context.Background(), gateway)).NotTo(HaveOccurred())
}

func GetTenantClientSet(tcp *kamajiv1alpha1.TenantControlPlane) (*kubernetes.Clientset, *os.File) {
	GinkgoHelper()

	var clientset *kubernetes.Clientset
	ctx := context.Background()

	kubeconfigFile, err := os.CreateTemp("", fmt.Sprintf("tcp-clientset-%s", string(tcp.ObjectMeta.UID)))
	Expect(err).ToNot(HaveOccurred())

	Eventually(func() (err error) {
		if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: tcp.GetNamespace(), Name: tcp.GetName()}, tcp); err != nil {
			_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: cannot retrieve TCP:", err.Error())

			return err
		}

		secret := &corev1.Secret{}

		if err = k8sClient.Get(ctx, types.NamespacedName{Namespace: tcp.GetNamespace(), Name: tcp.Status.KubeConfig.Admin.SecretName}, secret); err != nil {
			_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: cannot retrieve kubeconfig secret name:", err.Error())

			return err
		}

		_, err = kubeconfigFile.Write(secret.Data["admin.conf"])

		return err
	}, time.Minute, 5*time.Second).ShouldNot(HaveOccurred())

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigFile.Name())
	Expect(err).ToNot(HaveOccurred())

	clientset, err = kubernetes.NewForConfig(config)
	Expect(err).ToNot(HaveOccurred())

	return clientset, kubeconfigFile
}

func GetDaemonSetContainers(clientset *kubernetes.Clientset, namespace string, name string) []corev1.Container {
	var daemonSet *appsv1.DaemonSet
	var err error

	Eventually(func() error {
		daemonSet, err = clientset.AppsV1().DaemonSets(namespace).Get(context.Background(), name, metav1.GetOptions{})

		return err
	}).WithTimeout(1 * time.Minute).WithPolling(10 * time.Second).To(Succeed())

	return daemonSet.Spec.Template.Spec.Containers
}

func GetDeploymentContainers(clientset *kubernetes.Clientset, namespace string, name string) []corev1.Container {
	var deployment *appsv1.Deployment
	var err error

	Eventually(func() error {
		deployment, err = clientset.AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})

		return err
	}).WithTimeout(1 * time.Minute).WithPolling(10 * time.Second).To(Succeed())

	return deployment.Spec.Template.Spec.Containers
}

func CheckTemplateContainerEnvVars(clientset *kubernetes.Clientset, resourceKind string, resourceNamespace string, resourceName string, containerName string, expectedVars []corev1.EnvVar, only bool) {
	GinkgoHelper()

	var envVarMatcher gomegaTypes.GomegaMatcher

	if only {
		if len(expectedVars) > 0 {
			envVarMatcher = HaveExactElements(expectedVars)
		} else {
			envVarMatcher = Or(BeNil(), BeEmpty())
		}
	} else {
		envVarMatcher = ContainElements(expectedVars)
	}

	Eventually(func() []corev1.EnvVar {
		var containers []corev1.Container

		By("getting containers for ressource", func() {
			switch resourceKind {
			case "DaemonSet":
				containers = GetDaemonSetContainers(clientset, resourceNamespace, resourceName)
			case "Deployment":
				containers = GetDeploymentContainers(clientset, resourceNamespace, resourceName)
			default:
				containers = []corev1.Container{}
			}
		})

		var container corev1.Container

		By("checking for named container", func() {
			_, at := utilities.HasNamedContainer(containers, containerName)
			container = containers[at]
		})

		return container.Env
	}).WithTimeout(1 * time.Minute).WithPolling(10 * time.Second).To(envVarMatcher)
}

func CheckTCPContainerEnvVars(k8sClient client.Client, tcp kamajiv1alpha1.TenantControlPlane, containerName string, expectedVars []corev1.EnvVar, only bool) {
	GinkgoHelper()

	var envVarMatcher gomegaTypes.GomegaMatcher

	if only {
		if len(expectedVars) > 0 {
			envVarMatcher = HaveExactElements(expectedVars)
		} else {
			envVarMatcher = Or(BeNil(), BeEmpty())
		}
	} else {
		envVarMatcher = ContainElements(expectedVars)
	}

	Eventually(func() []corev1.EnvVar {
		var containers []corev1.Container

		By("getting containers for TCP deployment", func() {
			tcpDeployment := appsv1.Deployment{}
			Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: tcp.Name, Namespace: tcp.Namespace}, &tcpDeployment)).NotTo(HaveOccurred())

			containers = tcpDeployment.Spec.Template.Spec.Containers
		})

		var container corev1.Container

		By("checking for named container", func() {
			_, at := utilities.HasNamedContainer(containers, containerName)
			container = containers[at]
		})

		return container.Env
	}).WithTimeout(1 * time.Minute).WithPolling(10 * time.Second).To(envVarMatcher)
}
