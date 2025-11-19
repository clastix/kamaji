# Cluster Autoscaler

The [Cluster Autoscaler](https://github.com/kubernetes/autoscaler) is a tool that automatically adjusts the size of a Kubernetes cluster so that all pods have a place to run and no unneeded nodes remain.

When pods are unschedulable because there are not enough resources, the Cluster Autoscaler scales up the cluster. When nodes are underutilized, the Cluster Autoscaler scales the cluster down.

Cluster API supports the Cluster Autoscaler. See the [Cluster Autoscaler on Cluster API](https://cluster-api.sigs.k8s.io/tasks/automated-machine-management/autoscaling) for more information.

## Getting started with the Cluster Autoscaler on Kamaji

Kamaji supports the Cluster Autoscaler through Cluster API. There are several ways to run the Cluster Autoscaler with Cluster API. In this guide, we leverage the unique features of Kamaji to run the Cluster Autoscaler as part of the Hosted Control Plane.

In other words, the Cluster Autoscaler runs as a pod in the Kamaji Management Cluster, alongside the Tenant Control Plane pods, and connects directly to the API server of the workload cluster. This approach hides sensitive data from the tenant. It works by mounting the kubeconfig of the tenant cluster into the Cluster Autoscaler pod.

### Create the workload cluster

Create a workload cluster using the Kamaji Control Plane Provider and the Infrastructure Provider of your choice. The following example creates a workload cluster using the vSphere Infrastructure Provider.

The template file [`capi-kamaji-vsphere-autoscaler-template.yaml`](https://raw.githubusercontent.com/clastix/cluster-api-control-plane-provider-kamaji/master/templates/vsphere/capi-kamaji-vsphere-autoscaler-template.yaml) provides a full example of a cluster with the autoscaler enabled. You can generate the cluster manifest using `clusterctl`.

Before doing so, list all the variables in the template file:

```bash
cat capi-kamaji-vsphere-autoscaler-template.yaml | clusterctl generate yaml --list-variables
```

Fill them with the desired values and generate the manifest:

```bash
clusterctl generate yaml \
    --from capi-kamaji-vsphere-autoscaler-template.yaml \
    > capi-kamaji-vsphere-cluster.yaml
```

Apply the generated manifest to create the ClusterClass:

```bash
kubectl apply -f capi-kamaji-vsphere-cluster.yaml
```

### Install the Cluster Autoscaler

Install the Cluster Autoscaler via Helm in the Management Cluster, in the same namespace where the workload cluster is deployed.

!!! info "Options for installing the Cluster Autoscaler"
    The Cluster Autoscaler works on a single cluster, meaning every cluster must have its own Cluster Autoscaler instance. This can be addressed by leveraging Project Sveltos automations to deploy a Cluster Autoscaler instance for each Kamaji Cluster API instance.

```bash
helm repo add autoscaler https://kubernetes.github.io/autoscaler
helm repo update
helm upgrade --install ${CLUSTER_NAME}-autoscaler autoscaler/cluster-autoscaler \
    --set cloudProvider=clusterapi \
    --set autodiscvovery.namespace=default \
    --set "autoDiscovery.labels[0].autoscaling=enabled" \
    --set clusterAPIKubeconfigSecret=${CLUSTER_NAME}-kubeconfig \
    --set clusterAPIMode=kubeconfig-incluster
```

The `autoDiscovery.labels` values are used to dynamically select clusters to autoscale.

These labels must be set on the workload cluster, specifically in the `Cluster` and `MachineDeployment` resources.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: sample
    # Cluster Autoscaler labels
    autoscaling: enabled
  name: sample

# other fields omitted for brevity
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  annotations:
    # Cluster Autoscaler annotations
    cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size: "0"
    cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size: "6"
    capacity.cluster-autoscaler.kubernetes.io/cpu: "2"  # YMMV
    capacity.cluster-autoscaler.kubernetes.io/memory: 4Gi  # YMMV
    capacity.cluster-autoscaler.kubernetes.io/maxPods: "110"  # YMMV
  labels:
    cluster.x-k8s.io/cluster-name: sample
    # Cluster Autoscaler labels
    autoscaling: enabled
  name: sample-md-0

# other fields omitted for brevity
---
# other Cluster API resources omitted for brevity
```

### Verify the Cluster Autoscaler

To verify that the Cluster Autoscaler is working as expected, deploy a workload in the Tenant cluster with specific CPU requirements to simulate resource demand.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: hello-node
  name: hello-node
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: hello-node
  template:
    metadata:
      labels:
        app: hello-node
    spec:
      containers:
      - image: quay.io/google-containers/pause-amd64:3.0
        imagePullPolicy: IfNotPresent
        name: pause-amd64
        resources:
          limits:
            cpu: 500m
```

Apply the workload to the Tenant cluster and simulate a load spike by increasing the number of replicas. The Cluster Autoscaler should scale up the cluster to accommodate the workload. Cooldown times must be configured correctly on a per-cluster basis.

!!! warning "Possible Resource Wastage"
    With the Cluster Autoscaler, new machines may be created very quickly, which can lead to over-provisioning and potentially wasted resources. The official Cluster Autoscaler documentation should be consulted to configure appropriate values based on your infrastructure and provisioning times.

##  `ProvisioningRequest` support

The [ProvisioningRequest](https://github.com/kubernetes/autoscaler/blob/cluster-autoscaler-1.34.1/cluster-autoscaler/proposals/provisioning-request.md) introduces a Kubernetes-native way for Cluster Autoscaler to request new capacity without talking directly to cloud provider APIs.
Instead of embedding provider-specific logic, the autoscaler simply describes the capacity it needs, and an external provisioner decides how to create the required nodes.
This makes scaling portable across clouds, on-prem platforms, and custom provisioning systems, while greatly reducing complexity inside the autoscaler.

Once the cluster has been provisioned, install the `ProvisioningRequest` definition.

```
kubectl kamaji kubeconfig get capi-quickstart-kubevirt > /tmp/capi-quickstart-kubevirt
KUBECONFIG=/tmp/capi-quickstart-kubevirt kubectl apply -f https://raw.githubusercontent.com/kubernetes/autoscaler/refs/tags/cluster-autoscaler-1.34.1/cluster-autoscaler/apis/config/crd/autoscaling.x-k8s.io_provisioningrequests.yaml
```

Proceed with the installation of Cluster Autoscaler by enabling some additional parameters: YMMV.

```yaml
cloudProvider: clusterapi
autoDiscovery:
  namespace: default
  labels:
  - autoscaling.x-k8s.io: enabled

clusterAPIKubeconfigSecret: capi-quickstart-kubeconfig
clusterAPIMode: kubeconfig-incluster

extraArgs:
  enable-provisioning-requests: true
  kube-api-content-type: "application/json"
  cloud-config: /etc/kubernetes/management/kubeconfig

extraVolumeSecrets:
  # Mount the management kubeconfig to talk with the management cluster:
  # the in-rest configuration doesn't work
  management-kubeconfig:
    name: management-kubeconfig
    mountPath: /etc/kubernetes/management
    items:
    - key: kubeconfig
      path: kubeconfig
```

The Cluster Autoscaler should be up and running, enabled to connect to the management and tenant cluster API Server:
follow the [official example](https://github.com/kubernetes/autoscaler/blob/cluster-autoscaler-1.34.1/cluster-autoscaler/FAQ.md#example-usage) from the repository to assess the `ProvisioningRequest` feature.
