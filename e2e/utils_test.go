// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

func GetKindIPAddress() string {
	ep := &corev1.Endpoints{}
	Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: "kubernetes", Namespace: "default"}, ep)).ToNot(HaveOccurred())

	return ep.Subsets[0].Addresses[0].IP
}

func PrintKamajiLogs() {
	if CurrentGinkgoTestDescription().Failed {
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
				seconds := int64(CurrentGinkgoTestDescription().Duration.Seconds())

				return &seconds
			}(),
			Timestamps: true,
		})

		podLogs, err := request.Stream(context.Background())
		Expect(err).ToNot(HaveOccurred())

		defer podLogs.Close()

		podBytes, err := ioutil.ReadAll(podLogs)
		Expect(err).ToNot(HaveOccurred())

		_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: retrieving Kamaji Pod logs")

		for _, line := range bytes.Split(podBytes, []byte("\n")) {
			_, _ = fmt.Fprintln(GinkgoWriter, ">>> ", string(line))
		}

		_, _ = fmt.Fprintln(GinkgoWriter, "DEBUG: end of Kamaji Pod logs")
	}
}
