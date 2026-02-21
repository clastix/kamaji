# Kamaji and `externalClusterReference` usage

This document explains how to use **Kamaji's `externalClusterReference`** together with **Cluster API (CAPI)** to run Kubernetes control planes on an **external cluster**, while managing worker nodes from a management cluster.

It assumes the use of the KubeVirt infrastructure provider for ease of deployment and local testing.

---

## High-level Architecture

The following setup operates on **two Kubernetes clusters**:

- **Management cluster** – runs Cluster API controllers and the Kamaji control-plane provider, and manages cluster lifecycle and topology.
- **External cluster** - runs Kamaji, hosts the Kubernetes control plane components and receives control plane workloads via Kamaji

---

## Prerequisites

- `docker`
- `kind`
- `kubectl`
- `clusterctl`
- `helm`

---

## Step 1: Create the KIND clusters

Create the **management** cluster:

```bash
kind create cluster --name management
```

Create the **external** cluster that will host control planes:

```bash
kind create cluster --name external
```

Verify contexts:

```bash
kubectl config get-contexts
```

---

## Step 2: Initialize Cluster API controllers

Switch to the management cluster:

```bash
kubectl config use-context kind-management
```

Enable ClusterClass support and initialize Cluster API with Kamaji and KubeVirt:

```bash
export CLUSTER_TOPOLOGY=true
clusterctl init \
  --core cluster-api \
  --bootstrap kubeadm \
  --infrastructure kubevirt \
  --control-plane kamaji
```

---

## Step 3: Enable Kamaji external cluster feature gates

Patch the Kamaji controller to enable `externalClusterReference`:

```bash
kubectl -n kamaji-system patch deployment capi-kamaji-controller-manager \
  --type='json' \
  -p='[
    {
      "op": "replace",
      "path": "/spec/template/spec/containers/0/args/1",
      "value": "--feature-gates=ExternalClusterReference=true,ExternalClusterReferenceCrossNamespace=true"
    }
  ]'
```

---

## Step 4: Install KubeVirt

Fetch the latest stable KubeVirt version and install:

```bash
export VERSION=$(curl -s "https://storage.googleapis.com/kubevirt-prow/release/kubevirt/kubevirt/stable.txt")
kubectl apply -f "https://github.com/kubevirt/kubevirt/releases/download/${VERSION}/kubevirt-operator.yaml"
kubectl apply -f "https://github.com/kubevirt/kubevirt/releases/download/${VERSION}/kubevirt-cr.yaml"
```

Enable emulation (optional, if virtualization is not supported):

```bash
kubectl -n kubevirt patch kubevirt kubevirt \
  --type=merge \
  --patch '{"spec":{"configuration":{"developerConfiguration":{"useEmulation":true}}}}'
```

---

## Step 5: Prepare kubeconfig for the external cluster

Retrieve the external cluster control-plane address:

```bash
EXT_CP_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "external-control-plane")
```

Export and rewrite the kubeconfig:

```bash
kubectl --context kind-external config view --raw --minify --flatten > kind-external.kubeconfig
```

Replace the API endpoint with the cluster IP, required for cross-cluster access from the management cluster:

```bash
bash -c "sed -i -E 's#https://[^:]+:[0-9]+#https://$EXT_CP_IP:6443#g' kind-external.kubeconfig"
```

Create the kubeconfig secret in the management cluster:

```bash
kubectl -n default create secret generic kind-external-kubeconfig \
  --from-file=kubeconfig=kind-external.kubeconfig
```

---

## Step 6: Install Kamaji and dependencies on the external cluster

Switch context:

```bash
kubectl config use-context kind-external
```

Install cert-manager:

```bash
helm upgrade --install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --set installCRDs=true
```

Install Kamaji:

```bash
helm upgrade --install kamaji clastix/kamaji \
  --namespace kamaji-system \
  --create-namespace \
  --set 'resources=null' \
  --version 0.0.0+latest
```

Install MetalLB:

```bash
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.15.3/config/manifests/metallb-native.yaml
```

Configure MetalLB IP address pool:

```bash
SUBNET=$(docker network inspect kind | jq -r '.[0].IPAM.Config[] | select(.Subnet | test(":") | not) | .Subnet' | head -n1)
NET_PREFIX=$(echo "$SUBNET" | cut -d/ -f1 | awk -F. '{print $1"."$2}')
kubectl apply -f - <<EOF
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: default
  namespace: metallb-system
spec:
  addresses:
  - ${NET_PREFIX}.255.200-${NET_PREFIX}.255.250
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: default
  namespace: metallb-system
EOF
```

Create tenant namespace:

```bash
kubectl create namespace kamaji-tenants
```

---

## Step 7: Definition of KamajiControlPlaneTemplate

The `KamajiControlPlaneTemplate` is defined in [the following manifest](https://raw.githubusercontent.com/clastix/kamaji/master/config/capi/clusterclass-kubevirt-kamaji-external.yaml) and can be applied directly.

This template configures how Kamaji deploys and manages the tenant control plane on an external Kubernetes cluster using Cluster API.

```bash
apiVersion: controlplane.cluster.x-k8s.io/v1alpha1
kind: KamajiControlPlaneTemplate
metadata:
  name: kamaji-controlplane-external
  namespace: default
spec:
  template:
    spec:
      addons:
        coreDNS: {}
        konnectivity: {}
        kubeProxy: {}
      dataStoreName: "default" # reference to DataStore present on external cluster
      deployment:
        externalClusterReference:
          deploymentNamespace: kamaji-tenants
          kubeconfigSecretName: kind-external-kubeconfig
          kubeconfigSecretKey: kubeconfig
      network:
        serviceType: LoadBalancer
      kubelet:
        cgroupfs: systemd
        preferredAddressTypes:
        - InternalIP
      registry: "registry.k8s.io"
```

The `.spec.template.spec.deployment.externalClusterReference` section defines how Kamaji connects to and deploys control plane components into the external cluster:

- `deploymentNamespace` - The namespace on the external cluster where `TenantControlPlane` resources and control plane components are created.
- `kubeconfigSecretName` - The name of the Kubernetes Secret containing a kubeconfig that allows Kamaji to authenticate to the external cluster.
- `kubeconfigSecretKey` - The key inside the secret that holds the kubeconfig data.

The referenced secret must exist in the Kamaji management cluster and provide sufficient permissions to create and manage resources in the target external cluster.

---

## Step 8: Create the Cluster

Switch context back to the management cluster:

```bash
kubectl config use-context kind-management
```

Apply the Cluster manifest:

```bash
kubectl apply -f "https://raw.githubusercontent.com/clastix/kamaji/master/config/capi/clusterclass-kubevirt-kamaji-external.yaml"
```

---

## Validation

Check tenant control plane pods running in the external cluster:

```bash
kubectl --context kind-external -n kamaji-tenants get pods
```

Check cluster status in the management cluster:

```bash
kubectl --context kind-management get clusters
kubectl --context kind-management get kamajicontrolplanes
```

Get cluster kubeconfig and confirm it is working:

```bash
kubectl config use-context kind-management
clusterctl get kubeconfig demo-external > demo-external.kubeconfig
KUBECONFIG=./demo-external.kubeconfig kubectl get nodes
```

---

## Clean up

Delete Kind clusters:

```bash
kind delete cluster --name management
kind delete cluster --name external
```

---

## Summary

Using `externalClusterReference` with Kamaji and Cluster API enables:

- Hosted Kubernetes control planes on remote clusters
- Strong separation of concerns
- Multi-cluster management patterns
- Clean integration with ClusterClass
