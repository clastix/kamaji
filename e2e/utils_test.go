// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func GetKindIPAddress() string {
	ep := &corev1.Endpoints{}
	Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: "kubernetes", Namespace: "default"}, ep)).ToNot(HaveOccurred())

	return ep.Subsets[0].Addresses[0].IP
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
	Eventually(func() kamajiv1alpha1.KubernetesVersionStatus {
		err := k8sClient.Get(context.Background(), types.NamespacedName{
			Name:      tcp.GetName(),
			Namespace: tcp.GetNamespace(),
		}, tcp)
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

func PodsServiceAccountMustEqualTo(tcp *kamajiv1alpha1.TenantControlPlane, sa *corev1.ServiceAccount) {
    saName := sa.GetName()
    Eventually(func () bool {
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
    }, 5*time.Minute, time.Second).Should(Equal(true)) 
}
