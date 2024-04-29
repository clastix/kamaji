// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=etcd;MySQL;PostgreSQL;NATS

type Driver string

var (
	EtcdDriver           Driver = "etcd"
	KineMySQLDriver      Driver = "MySQL"
	KinePostgreSQLDriver Driver = "PostgreSQL"
	KineNatsDriver       Driver = "NATS"
)

// +kubebuilder:validation:MinItems=1

type Endpoints []string

// DataStoreSpec defines the desired state of DataStore.
type DataStoreSpec struct {
	// The driver to use to connect to the shared datastore.
	Driver Driver `json:"driver"`
	// List of the endpoints to connect to the shared datastore.
	// No need for protocol, just bare IP/FQDN and port.
	Endpoints Endpoints `json:"endpoints"`
	// In case of authentication enabled for the given data store, specifies the username and password pair.
	// This value is optional.
	BasicAuth *BasicAuth `json:"basicAuth,omitempty"`
	// Defines the TLS/SSL configuration required to connect to the data store in a secure way.
	// This value is optional.
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`
}

// TLSConfig contains the information used to connect to the data store using a secured connection.
type TLSConfig struct {
	// Retrieve the Certificate Authority certificate and private key, such as bare content of the file, or a SecretReference.
	// The key reference is required since etcd authentication is based on certificates, and Kamaji is responsible in creating this.
	CertificateAuthority CertKeyPair `json:"certificateAuthority"`
	// Specifies the SSL/TLS key and private key pair used to connect to the data store.
	ClientCertificate *ClientCertificate `json:"clientCertificate,omitempty"`
}

type ClientCertificate struct {
	Certificate ContentRef `json:"certificate"`
	PrivateKey  ContentRef `json:"privateKey"`
}

type CertKeyPair struct {
	Certificate ContentRef  `json:"certificate"`
	PrivateKey  *ContentRef `json:"privateKey,omitempty"`
}

// BasicAuth contains the required information to perform the connection using user credentials to the data store.
type BasicAuth struct {
	Username ContentRef `json:"username"`
	Password ContentRef `json:"password"`
}

type ContentRef struct {
	// Bare content of the file, base64 encoded.
	// It has precedence over the SecretReference value.
	Content   []byte           `json:"content,omitempty"`
	SecretRef *SecretReference `json:"secretReference,omitempty"`
}

// +kubebuilder:validation:MinLength=1
type secretReferKeyPath string

type SecretReference struct {
	corev1.SecretReference `json:",inline"`
	// Name of the key for the given Secret reference where the content is stored.
	// This value is mandatory.
	KeyPath secretReferKeyPath `json:"keyPath"`
}

// DataStoreStatus defines the observed state of DataStore.
type DataStoreStatus struct {
	// List of the Tenant Control Planes, namespaced named, using this data store.
	UsedBy []string `json:"usedBy,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="Driver",type="string",JSONPath=".spec.driver",description="Kamaji data store driver"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Age"

// DataStore is the Schema for the datastores API.
type DataStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DataStoreSpec   `json:"spec,omitempty"`
	Status DataStoreStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DataStoreList contains a list of DataStore.
type DataStoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DataStore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DataStore{}, &DataStoreList{})
}
