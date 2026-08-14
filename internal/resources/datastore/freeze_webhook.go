// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pointer "k8s.io/utils/ptr"
)

// FreezeWebhookName is the name of the ValidatingWebhookConfiguration installed in the tenant
// cluster to block writes while a datastore migration is in progress.
const FreezeWebhookName = "kamaji-freeze"

// BuildFreezeValidatingWebhookConfiguration returns the desired Webhooks for the tenant-side
// "kamaji-freeze" ValidatingWebhookConfiguration. Shared by the host-side Migrate resource,
// which installs it synchronously before starting the migration Job, and the soot Migrate
// controller, which removes it once the TenantControlPlane is Ready again.
func BuildFreezeValidatingWebhookConfiguration(namespace, serviceName string, caBundle []byte) []admissionregistrationv1.ValidatingWebhook {
	url := pointer.To(fmt.Sprintf("https://%s.%s.svc:443/migrate", serviceName, namespace))

	return []admissionregistrationv1.ValidatingWebhook{
		{
			Name: "leases.migrate.kamaji.clastix.io",
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				URL:      url,
				CABundle: caBundle,
			},
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Delete,
					},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"*"},
						APIVersions: []string{"*"},
						Resources:   []string{"*"},
						Scope:       pointer.To(admissionregistrationv1.NamespacedScope),
					},
				},
			},
			FailurePolicy: pointer.To(admissionregistrationv1.Fail),
			MatchPolicy:   pointer.To(admissionregistrationv1.Equivalent),
			NamespaceSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "kubernetes.io/metadata.name",
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{
							"kube-node-lease",
						},
					},
				},
			},
			SideEffects:             pointer.To(admissionregistrationv1.SideEffectClassNoneOnDryRun),
			AdmissionReviewVersions: []string{"v1"},
		},
		{
			Name: "catchall.migrate.kamaji.clastix.io",
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				URL:      url,
				CABundle: caBundle,
			},
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"*"},
						APIVersions: []string{"*"},
						Resources:   []string{"*"},
						Scope:       pointer.To(admissionregistrationv1.AllScopes),
					},
				},
			},
			FailurePolicy: pointer.To(admissionregistrationv1.Fail),
			MatchPolicy:   pointer.To(admissionregistrationv1.Equivalent),
			NamespaceSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "kubernetes.io/metadata.name",
						Operator: metav1.LabelSelectorOpNotIn,
						Values: []string{
							"kube-system",
							"kube-node-lease",
						},
					},
				},
			},
			SideEffects:             pointer.To(admissionregistrationv1.SideEffectClassNoneOnDryRun),
			TimeoutSeconds:          nil,
			AdmissionReviewVersions: []string{"v1"},
		},
	}
}
