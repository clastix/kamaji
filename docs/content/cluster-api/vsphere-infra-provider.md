# vSphere Infrastructure Provider

Use the [vSphere Infrastructure Provider](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere) to create a fully functional Kubernetes cluster on **vSphere** using the [Kamaji Control Plane Provider](https://github.com/clastix/cluster-api-control-plane-provider-kamaji).

!!! info "Virtual Machines Placement"
    As Kamaji decouples the Control Plane from the infrastructure, the Kamaji Management Cluster hosting the Tenant control Plane, is not required to be on the same vSphere where worker machines will be. As network reachability is satisfied, you can have your Kamaji Management Cluster on a different vSphere or even on a different cloud provider.

## vSphere Requirements

You need to access a **vSphere** environment with the following requirements:

- The vSphere environment should be configured with a DHCP service in the primary VM network for your tenant clusters. Alternatively you can use an [IPAM Provider](https://github.com/kubernetes-sigs/cluster-api-ipam-provider-in-cluster).

- Configure one Resource Pool across the hosts onto which the tenant clusters will be provisioned. Every host in the Resource Pool will need access to a shared storage.

- A Template VM based on published [OVA images](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere). For production-like environments, it is highly recommended to build and use your own custom OVA images. Take a look to the [image-builder](https://github.com/kubernetes-sigs/image-builder) project.

- To use the vSphere Container Storage Interface (CSI), your vSphere cluster needs support for Cloud Native Storage (CNS). CNS relies on a shared datastore. Ensure that your vSphere environment is properly configured to support CNS.

## Install the vSphere Infrastructure Provider

In order to use vSphere Cluster API provider, you must be able to connect and authenticate to a **vCenter**. Ensure you have credentials to your vCenter server:

```bash
export VSPHERE_USERNAME="admin@vsphere.local"
export VSPHERE_PASSWORD="*******"
```

Install the vSphere Infrastructure Provider:

```bash
clusterctl init --infrastructure vsphere
```

## Install the IPAM Provider

If you intend to use IPAM to assign addresses to the nodes, you can use the in-cluster [IPAM provider](https://github.com/kubernetes-sigs/cluster-api-ipam-provider-in-cluster) instead of rely on DHCP service. To do so, initialize the Management Cluster with the `--ipam in-cluster` flag:

```bash
clusterctl init --ipam in-cluster
```

## Create a Tenant Cluster

Once all the controllers are up and running in the management cluster, you can apply the cluster manifests containing the specifications of the tenant cluster you want to provision.

### Generate the Cluster Manifest using the template

Using `clusterctl`, you can generate a tenant cluster manifest for your vSphere environment. Set the environment variables to match your vSphere configuration:

```bash
# VSphere Configuration
export VSPHERE_USERNAME="admin@vsphere.local"
export VSPHERE_PASSWORD="changeme"
export VSPHERE_SERVER="vcenter.vsphere.local"
export VSPHERE_DATACENTER: "SDDC-Datacenter"
export VSPHERE_DATASTORE: "DefaultDatastore"
export VSPHERE_NETWORK: "VM Networkt"
export VSPHERE_RESOURCE_POOL: "*/Resources"
export VSPHERE_FOLDER: "kamaji-capi-pool"
export VSPHERE_TEMPLATE: "ubuntu-2404-kube-v1.31.0"
export VSPHERE_TLS_THUMBPRINT: "..."
export VSPHERE_STORAGE_POLICY: ""
export KUBERNETES_VERSION: "v1.31.0"
export CPI_IMAGE_K8S_VERSION: "v1.31.0"
export CSI_INSECURE: "1"
export VSPHERE_SSH_USER: "clastix"
export VSPHERE_SSH_AUTHORIZED_KEY: "ssh-rsa AAAAB3N..." 
```

If you intend to use IPAM, set the environment variables to match your IPAM configuration:

```bash
# IPAM Configuration
export NODE_IPAM_POOL_API_GROUP="ipam.cluster.x-k8s.io"
export NODE_IPAM_POOL_KIND="InClusterIPPool"
export NODE_IPAM_POOL_NAME="ipam-ip-pool"
export NODE_IPAM_POOL_RANGE="10.9.62.100-10.9.62.200"
export NODE_IPAM_POOL_PREFIX="24"
export NODE_IPAM_POOL_GATEWAY="10.9.62.1"
```

Set the environment variables to match your cluster configuration:

```bash
# Cluster Configuration
export CLUSTER_NAME="sample"
export CLUSTER_NAMESPACE="default"
export POD_CIDR="10.36.0.0/16"
export SVC_CIDR="10.96.0.0/16"
export CONTROL_PLANE_REPLICAS=2
export CONTROL_PLANE_ENDPOINT_IP="10.9.62.30"
export KUBERNETES_VERSION="v1.31.0"
export CPI_IMAGE_K8S_VERSION="v1.31.0"
export CSI_INSECURE="1"
export NODE_DISK_SIZE=25
export NODE_MEMORY_SIZE=8192
export NODE_CPU_COUNT=2
export MACHINE_DEPLOY_REPLICAS=3
export NAMESERVER="8.8.8.8"
```

The following command will generate a cluster manifest based on the [`capi-kamaji-vsphere-template.yaml`](https://raw.githubusercontent.com/clastix/cluster-api-control-plane-provider-kamaji/main/templates/capi-kamaji-vsphere-template.yaml) template file:

```bash
clusterctl generate cluster --from capi-kamaji-vsphere-template.yaml > capi-kamaji-vsphere-cluster.yaml
```

### Apply the Cluster Manifest

Apply the generated cluster manifest to create the tenant cluster:

```bash
kubectl apply -f capi-kamaji-vsphere-cluster.yaml
```

You can check the status of the cluster deployment with `clusterctl`:

```bash
clusterctl describe cluster sample

NAME                                                       READY  SEVERITY  REASON  SINCE  MESSAGE
Cluster/sample                                             True                     33m
├─ClusterInfrastructure - VSphereCluster/sample            True                     34m
├─ControlPlane - KamajiControlPlane/sample                 True                     34m
└─Workers
  └─MachineDeployment/sample-md-0                          True                     80s
    └─3 Machines...                                        True                     32m    See ...
```

A new tenant cluster named `sample` is created with a Tenant Control Plane and three worker nodes. You can check the status of the tenant cluster with `kubectl`:

```bash
kubectl get clusters -n default
```

and related tenant control plane created on Kamaji Management Cluster:

```bash
kubectl get tcp -n default
```

## Access the Tenant Cluster

To access the tenant cluster, you can estract the `kubeconfig` file from the Kamaji Management Cluster:

```bash
kubectl get secret sample-kubeconfig -o jsonpath='{.data.value}' | base64 -d > ~/.kube/sample.kubeconfig
```

and use it to access the tenant cluster:

```bash
export KUBECONFIG=~/.kube/sample.kubeconfig
kubectl cluster-info
```

## Cloud Controller Manager

The template file `capi-kamaji-vsphere-template.yaml` includes the external [Cloud Controller Manager (CCM)](https://github.com/kubernetes/cloud-provider-vsphere) configuration for vSphere. The CCM is a Kubernetes controller that manages the cloud provider's resources. The CCM is responsible for creating and managing the cloud provider's resources, such as Load Balancers, Persistent Volumes, and Node Balancers.

## Delete the Tenant Cluster

For cluster deletion, use the following command:

```bash
kubectl delete cluster sample
```

!!! warning "Orphan Resources"
    Do NOT use `kubectl delete -f capi-kamaji-vsphere-cluster.yaml` as that can result in orphan resources. Always use `kubectl delete cluster sample` to delete the tenant cluster.