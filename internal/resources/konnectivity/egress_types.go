// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Local type definitions for EgressSelectorConfiguration to avoid importing
// k8s.io/apiserver which conflicts with controller-runtime workqueue metrics.
// These types are based on k8s.io/apiserver/pkg/apis/apiserver/v1alpha1.

// ProtocolType is the type of the proxy protocol used for egress selection.
type ProtocolType string

const (
	// ProtocolHTTPConnect uses HTTP CONNECT as the proxy protocol.
	ProtocolHTTPConnect ProtocolType = "HTTPConnect"
	// ProtocolGRPC uses GRPC as the proxy protocol.
	ProtocolGRPC ProtocolType = "GRPC"
	// ProtocolDirect establishes a direct connection without proxy.
	ProtocolDirect ProtocolType = "Direct"
)

// +kubebuilder:object:root=true
// +kubebuilder:object:generate=true

// EgressSelectorConfiguration provides versioned configuration for egress selector clients.
type EgressSelectorConfiguration struct {
	metav1.TypeMeta  `json:",inline"`
	EgressSelections []EgressSelection `json:"egressSelections"`
}

// +kubebuilder:object:generate=true

// EgressSelection provides the configuration for a single egress selection client.
type EgressSelection struct {
	Name       string     `json:"name"`
	Connection Connection `json:"connection"`
}

// +kubebuilder:object:generate=true

// Connection provides the configuration for a single egress selection client connection.
type Connection struct {
	ProxyProtocol ProtocolType `json:"proxyProtocol,omitempty"`
	Transport     *Transport   `json:"transport,omitempty"`
}

// +kubebuilder:object:generate=true

// Transport defines the transport configurations we support for egress selector.
type Transport struct {
	TCP *TCPTransport `json:"tcp,omitempty"`
	UDS *UDSTransport `json:"uds,omitempty"`
}

// +kubebuilder:object:generate=true

// TCPTransport provides the information to connect to a TCP endpoint.
type TCPTransport struct {
	URL string `json:"url,omitempty"`
}

// +kubebuilder:object:generate=true

// UDSTransport provides the information to connect to a Unix Domain Socket endpoint.
type UDSTransport struct {
	UDSName string `json:"udsName,omitempty"`
}
