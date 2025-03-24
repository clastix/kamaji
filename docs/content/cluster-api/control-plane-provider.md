# Kamaji Control Plane Provider

Kamaji can act as a Cluster API Control Plane provider using the `KamajiControlPlane` custom resource, which defines the control plane of a Tenant Cluster.

Here is an example of a `KamajiControlPlane`:

```yaml
kind: KamajiControlPlane
apiVersion: controlplane.cluster.x-k8s.io/v1alpha1
metadata:
  name: '${CLUSTER_NAME}'
  namespace: '${CLUSTER_NAMESPACE}'
spec:
  apiServer:
    extraArgs:
      - --cloud-provider=external
  controllerManager:
    extraArgs:
      - --cloud-provider=external
  dataStoreName: default
  addons:
    coreDNS: {}
    kubeProxy: {}
    konnectivity: {}
  kubelet:
    cgroupfs: systemd
    preferredAddressTypes:
      - InternalIP
  network:
    serviceType: LoadBalancer
  version: ${KUBERNETES_VERSION}
```

You can use this as reference in a standard `Cluster` custom resource as controlplane provider:

```yaml
kind: Cluster
apiVersion: cluster.x-k8s.io/v1beta1
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: '${CLUSTER_NAME}'
  name: '${CLUSTER_NAME}'
  namespace: '${CLUSTER_NAMESPACE}'
spec:
  controlPlaneRef:
    apiVersion: controlplane.cluster.x-k8s.io/v1beta1
    kind: KamajiControlPlane
    name: '${CLUSTER_NAME}'
  clusterNetwork:
    pods:
      cidrBlocks:
        - '${PODS_CIDR}'
    services:
      cidrBlocks:
        - '${SERVICES_CIDR}'
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: ... # your infrastructure kind may vary
    name: '${CLUSTER_NAME}'
```

!!! info "Full Reference"
    For a full reference of the `KamajiControlPlane` custom resource, please see the [Reference APIs](https://doc.crds.dev/github.com/clastix/cluster-api-control-plane-provider-kamaji/controlplane.cluster.x-k8s.io/KamajiControlPlane/v1alpha1).

## Getting started with the Kamaji Control Plane Provider

Cluster API Provider Kamaji is compliant with the `clusterctl` contract, which means you can use it with the `clusterctl` CLI to create and manage your Kamaji based clusters.

!!! info "Options for install Cluster API"
    There are two ways to getting started with Cluster API:

    * using `clusterctl` to install the Cluster API components.
    * using the Cluster API Operator. Please refer to the [Cluster API Operator](https://cluster-api-operator.sigs.k8s.io/) guide for this option.

### Prerequisites

* [`clusterctl`](https://cluster-api.sigs.k8s.io/user/quick-start#install-clusterctl) installed in your workstation to handle the lifecycle of your clusters.
* [`kubectl`](https://kubernetes.io/docs/tasks/tools/) installed in your workstation to interact with your clusters.
* [Kamaji](../getting-started/getting-started.md) installed in your Management Cluster.

### Initialize the Management Cluster

Use `clusterctl` to initialize the Management Cluster. When executed for the first time, `clusterctl init` will fetch and install the Cluster API components in the Management Cluster

```bash
clusterctl init --control-plane kamaji
```

As result, the following Cluster API components will be installed:

* Cluster API Provider in `capi-system` namespace
* Bootstrap Provider in `capi-kubeadm-bootstrap-system` namespace
* Kamaji Control Plane Provider in `kamaji-system` namespace

In the next step, we will create a fully functional Kubernetes cluster using the Kamaji Control Plane Provider and the Infrastructure provider of choice.

For a complete list of supported infrastructure providers, please refer to the [other providers](other-providers.md) page.

