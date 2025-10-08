# Tenant Control Plane


Kamaji introduces a new way to manage Kubernetes control planes at scale. Instead of dedicating separate machines to each cluster’s control plane, Kamaji runs every Tenant Cluster’s control plane as a set of pods inside the Management Cluster. This design unlocks significant efficiencies: you can operate hundreds or thousands of isolated Kubernetes clusters on shared infrastructure, all while maintaining strong separation and reliability.

At the heart of this approach is Kamaji’s commitment to upstream compatibility. The control plane components—`kube-apiserver`, `kube-scheduler`, and `kube-controller-manager`—are the same as those used in any CNCF-compliant Kubernetes cluster. Kamaji uses `kubeadm` for setup and lifecycle management, so you get the benefits of a standard, certified Kubernetes experience.

## How It Works

When you want to create a new Tenant Cluster, you simply define a `TenantControlPlane` resource in the Management Cluster. Kamaji’s controllers take over from there, deploying the necessary control plane pods, configuring networking, and connecting to the appropriate datastore. The control plane is exposed via a Kubernetes Service—by default as a `LoadBalancer`, but you can also use `NodePort` or `ClusterIP` depending on your needs.

Worker nodes, whether virtual machines or bare metal, join the Tenant Cluster by connecting to its control plane endpoint. This process is compatible with standard Kubernetes tools and can be automated using Cluster API or other infrastructure automation solutions.

## Highlights

- **Efficiency and Scale:**  
  By running control planes as pods, Kamaji reduces the infrastructure and operational overhead of managing many clusters.

- **High Availability and Automation:**  
  Control plane pods are managed by Kubernetes Deployments, enabling rolling updates, self-healing, and autoscaling. Kamaji automates the entire lifecycle, from creation to deletion.

- **Declarative and GitOps:**  
  The `TenantControlPlane` custom resource allows you to manage clusters declaratively, fitting perfectly with GitOps and Infrastructure as Code workflows.

- **Seamless Integration:**  
  Kamaji works with Cluster API, supports a variety of datastores, and is compatible with the full Kubernetes ecosystem.

Kamaji's Tenant Control Plane model is designed for organizations that need to deliver robust, production-grade Kubernetes clusters at scale—whether for internal platform engineering, managed services, or multi-tenant environments.

## Certificate Authority Configuration

Kamaji supports configuring internal Certificate Authorities for TenantControlPlane components. This is particularly important in enterprise environments where control plane components need to trust internal certificate authorities for proper certificate validation.

### Internal CA Certificates

When TenantControlPlane components need to communicate with internal services (such as corporate identity providers, monitoring systems, or internal APIs) that use certificates signed by internal Certificate Authorities, you can configure Kamaji to mount these CA certificates.

Use the `internalCACertificatesConfigMap` field in the TenantControlPlane spec to reference a ConfigMap containing your organization's internal CA certificates:

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: my-cluster
spec:
  # ... other configuration
  internalCACertificatesConfigMap: "internal-ca-certs"
```

The referenced ConfigMap should contain CA certificates in PEM format:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: internal-ca-certs
  namespace: default
data:
  internal-ca.crt: |
    -----BEGIN CERTIFICATE-----
    # Your internal CA certificate content goes here
    -----END CERTIFICATE-----
```

When configured, Kamaji will mount these certificates to standard CA certificate paths in all control plane containers:
- `/etc/ca-certificates`
- `/usr/share/ca-certificates`
- `/usr/local/share/ca-certificates`

This resolves "x509: certificate signed by unknown authority" errors that can occur when control plane components interact with internal services.

