# Cluster Class

Kamaji supports **ClusterClass**, a simple way to create many clusters of a similar shape. This is useful for creating many clusters with the same configuration, such as a development cluster, a staging cluster, and a production cluster.

!!! warning "Caution!"
    ClusterClass is an experimental feature of Cluster API. As with any experimental features it should be used with caution as it may be unreliable. All experimental features are not subject to any compatibility or deprecation policy and are not yet recommended for production use.

You can read more about ClusterClass in the [Cluster API documentation](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/).

## Enabling ClusterClass

To enable ClusterClass, you need to set `CLUSTER_TOPOLOGY` before running `clusterctl init`. This will enable the Cluster API feature gate for ClusterClass.

```bash
export CLUSTER_TOPOLOGY=true
clusterctl init --infrastructure vsphere --control-plane kamaji
```

## Creating a ClusterClass

To create a ClusterClass, you need to create a `ClusterClass` custom resource. Here is an example of a `ClusterClass` that will create a cluster running control plane on the Kamaji Management Cluster and worker nodes on vSphere:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: ClusterClass
metadata:
  name: kamaji-clusterclass
spec:
  controlPlane:
    ref:
      apiVersion: controlplane.cluster.x-k8s.io/v1alpha1
      kind: KamajiControlPlaneTemplate
      name: kamaji-clusterclass-kamaji-control-plane-template
  infrastructure:
    ref:
      apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
      kind: VSphereClusterTemplate
      name: kamaji-clusterclass-vsphere-cluster-template
  workers:
    machineDeployments:
    - class: kamaji-clusterclass
      template:
        bootstrap:
          ref:
            apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
            kind: KubeadmConfigTemplate
            name: kamaji-clusterclass-kubeadm-config-template
        infrastructure:
          ref:
            apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
            kind: VSphereMachineTemplate
            name: kamaji-clusterclass-vsphere-machine-template

# other resources omitted for brevity ...
```

The template file [`capi-kamaji-vsphere-class-template.yaml`](https://raw.githubusercontent.com/clastix/cluster-api-control-plane-provider-kamaji/master/templates/capi-kamaji-vsphere-class-template.yaml) provides a full example of a ClusterClass for vSphere. You can generate a ClusterClass manifest using `clusterctl`.

Before you need to list all the variables in the template file:

```bash
cat capi-kamaji-vsphere-class-template.yaml | clusterctl generate yaml --list-variables
```

Fill them with the desired values and generate the manifest:

```bash
clusterctl generate yaml \
    --from capi-kamaji-vsphere-class-template.yaml \
    > capi-kamaji-vsphere-class.yaml
```

Apply the generated manifest to create the ClusterClass:

```bash
kubectl apply -f capi-kamaji-vsphere-class.yaml
```

## Creating a Cluster from a ClusterClass

Once a ClusterClass is created, you can create a Cluster using the ClusterClass. Here is an example of a Cluster that uses the `kamaji-clusterclass`:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: sample
spec:
  topology:
    class: kamaji-clusterclass
    classNamespace: capi-clusterclass
    version: v1.31.0
    controlPlane:
      replicas: 2
    workers:
      machineDeployments:
      - class: kamaji-clusterclass
        name: md-sample
        replicas: 3

# other resources omitted for brevity ...
```

Always refer to the [Cluster API documentation](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/) for the most up-to-date information on ClusterClass.