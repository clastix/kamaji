// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ManagedByLabel  = "kamaji.clastix.io/managed-by"
	ManagedForLabel = "kamaji.clastix.io/managed-for"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Age"
//+kubebuilder:metadata:annotations={"cert-manager.io/inject-ca-from=kamaji-system/kamaji-serving-cert"}
//+kubebuilder:resource:scope=Cluster,shortName=kc,categories=kamaji

// KubeconfigGenerator is the Schema for the kubeconfiggenerators API.
type KubeconfigGenerator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeconfigGeneratorSpec   `json:"spec,omitempty"`
	Status KubeconfigGeneratorStatus `json:"status,omitempty"`
}

// CompoundValue allows defining a static, or a dynamic value.
// Options are mutually exclusive, just one should be picked up.
// +kubebuilder:validation:XValidation:rule="(has(self.stringValue) || has(self.fromDefinition)) && !(has(self.stringValue) && has(self.fromDefinition))",message="Either stringValue or fromDefinition must be set, but not both."
type CompoundValue struct {
	// StringValue is a static string value.
	StringValue string `json:"stringValue,omitempty"`
	// FromDefinition is used to generate a dynamic value,
	// it uses the dot notation to access fields from the referenced TenantControlPlane object:
	// e.g.: metadata.name
	FromDefinition string `json:"fromDefinition,omitempty"`
}

type KubeconfigGeneratorSpec struct {
	// NamespaceSelector is used to filter Namespaces from which the generator should extract TenantControlPlane objects.
	NamespaceSelector metav1.LabelSelector `json:"namespaceSelector,omitempty"`
	// TenantControlPlaneSelector is used to filter the TenantControlPlane objects that should be address by the generator.
	TenantControlPlaneSelector metav1.LabelSelector `json:"tenantControlPlaneSelector,omitempty"`
	// Groups is resolved a set of strings used to assign the x509 organisations field.
	// It will be recognised by Kubernetes as user groups.
	Groups []CompoundValue `json:"groups,omitempty"`
	// User resolves to a string to identify the client, assigned to the x509 Common Name field.
	User CompoundValue `json:"user"`
	// ControlPlaneEndpointFrom is the key used to extract the Tenant Control Plane endpoint that must be used by the generator.
	// The targeted Secret is the `${TCP}-admin-kubeconfig` one, default to `admin.svc`.
	//+kubebuilder:default="admin.svc"
	ControlPlaneEndpointFrom string `json:"controlPlaneEndpointFrom,omitempty"`
}

type KubeconfigGeneratorStatusError struct {
	// Resource is the Namespaced name of the errored resource.
	//+kubebuilder:validation:Required
	Resource string `json:"resource"`
	// Message is the error message recorded upon the last generator run.
	//+kubebuilder:validation:Required
	Message string `json:"message"`
}

// KubeconfigGeneratorStatus defines the observed state of KubeconfigGenerator.
type KubeconfigGeneratorStatus struct {
	// Resources is the sum of targeted TenantControlPlane objects.
	//+kubebuilder:default=0
	Resources int `json:"resources"`
	// AvailableResources is the sum of successfully generated resources.
	// In case of a different value compared to Resources, check the field errors.
	//+kubebuilder:default=0
	AvailableResources int `json:"availableResources"`
	// Errors is the list of failed kubeconfig generations.
	Errors []KubeconfigGeneratorStatusError `json:"errors,omitempty"`
}

//+kubebuilder:object:root=true

// KubeconfigGeneratorList contains a list of TenantControlPlane.
type KubeconfigGeneratorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeconfigGenerator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubeconfigGenerator{}, &KubeconfigGeneratorList{})
}
