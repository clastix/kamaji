# Cluster Autoscaler

The [Cluster Autoscaler](https://github.com/kubernetes/autoscaler) is a tool that automatically adjusts the size of a Kubernetes cluster so that all pods have a place to run and there are no unneeded nodes.

When pods are unschedulable because there are not enough resources, Cluster Autoscaler scales up the cluster. When nodes are underutilized, Cluster Autoscaler scales down the cluster.

Cluster API supports the Cluster Autoscaler. See the [Cluster Autoscaler on Cluster API](https://cluster-api.sigs.k8s.io/tasks/automated-machine-management/autoscaling) for more information.

## Getting started with the Cluster Autoscaler on Kamaji

Kamaji supports the Cluster Autoscaler through Cluster API. There are several way to run the Cluster autoscaler with Cluster API. In this guide, we're leveraging the unique features of Kamaji to run the Cluster Autoscaler as part of Hosted Control Plane.

In other words, the Cluster Autoscaler is running as a pod in the Kamaji Management Cluster, side by side with the Tenant Control Plane pods, and connecting directly to the apiserver of the workload cluster, hiding sensitive data and information from the tenant: this can be done by mounting the kubeconfig of the tenant cluster in the Cluster Autoscaler pod.

### Create the workload cluster

Create a workload cluster using the Kamaji Control Plane Provider and the Infrastructure Provider of choice. The following example creates a workload cluster using the vSphere Infrastructure Provider:

The template file [`capi-kamaji-vsphere-autoscaler-template.yaml`](https://raw.githubusercontent.com/clastix/cluster-api-control-plane-provider-kamaji/master/templates/capi-kamaji-vsphere-autoscaler-template.yaml) provides a full example of a cluster with autoscaler enabled. You can generate the cluster manifest using `clusterctl`.

Before you need to list all the variables in the template file:

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

Install the Cluster Autoscaler via Helm in the Management Cluster, in the same namespace where workload cluster is deployed.

!!! info "Options for install Cluster Autoscaler"
    Cluster Autoscaler works on a single cluster: it means every cluster must have its own Cluster Autoscaler instance. This could be solved by leveraging on Project Sveltos automations, by deploying a Cluster Autoscaler instance for each Kamaji Cluster API instance.

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

The `autoDiscovery.labels` values are used to pick dynamically clusters to autoscale.

Such labels must be set on the workload cluster, in the `Cluster` and `MachineDeployment` resources.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
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

To verify the Cluster Autoscaler is working as expected, you can deploy a workload in the Tenant cluster with some CPU requirements in order to simulate workload requiring resources.

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

Apply the workload to the Tenant cluster and simulate the load spike by increasing the replicas. The Cluster Autoscaler should scale up the cluster to accommodate the workload. Cooldown time must be configured properly on a cluster basis. 

!!! warning "Possible Resource Wasting"
    With Cluster Autoscaler, new machines are automatically created in a very short time, ending up with some up-provisioning and potentially wasting resources. The official Cluster Autosclaler documentation must be understood to provide correct values according to the infrastructure and provisioning times.