# vSphere Infra Provider

Use the [vSphere Infrastructure Provider](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere) to create a fully functional Kubernetes cluster on **vSphere** using the [Kamaji Control Plane Provider](https://github.com/clastix/cluster-api-control-plane-provider-kamaji).

!!! info "Control Plane and Infrastructure Decoupling"
    Kamaji decouples the Control Plane from the infrastructure, so the Kamaji Management Cluster hosting the Tenant Control Plane does not need to be on the same vSphere as the worker machines. As long as network reachability is satisfied, you can have your Kamaji Management Cluster on a different vSphere or even on a different cloud provider.

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

Once all the controllers are up and running in the management cluster, you can generate and apply the cluster manifests of the tenant cluster you want to provision.

### Generate the Cluster Manifest using the template

Using `clusterctl`, you can generate a tenant cluster manifest for your vSphere environment. Set the environment variables to match your vSphere configuration.

For example:

```bash
# vSphere Configuration
export VSPHERE_SERVER="vcenter.vsphere.local"
export VSPHERE_DATACENTER="SDDC-Datacenter"
export VSPHERE_DATASTORE="DefaultDatastore"
export VSPHERE_NETWORK="VM Network"
export VSPHERE_RESOURCE_POOL="*/Resources"
export VSPHERE_FOLDER="kamaji-capi-pool"
export VSPHERE_TLS_THUMBPRINT="..."
export VSPHERE_STORAGE_POLICY="vSAN Storage Policy"
```

If you intend to use IPAM, set the environment variables to match your IPAM configuration.

For example:

```bash
# IPAM Configuration
export NODE_IPAM_POOL_RANGE="10.9.62.100-10.9.62.200"
export NODE_IPAM_POOL_PREFIX="24"
export NODE_IPAM_POOL_GATEWAY="10.9.62.1"
```

Set the environment variables to match your cluster configuration.

For example:

```bash
# Cluster Configuration
export CLUSTER_NAME="sample"
export CLUSTER_NAMESPACE="default"
export POD_CIDR="10.36.0.0/16"
export SVC_CIDR="10.96.0.0/16"
export CONTROL_PLANE_REPLICAS=2
export NAMESERVER="8.8.8.8"
export KUBERNETES_VERSION="v1.31.0"
export CPI_IMAGE_VERSION="v1.31.0"
```

Set the environment variables to match your machine configuration.

For example:

```bash
# Machine Configuration
export MACHINE_TEMPLATE="ubuntu-2404-kube-v1.31.0"
export MACHINE_DEPLOY_REPLICAS=2
export NODE_DISK_SIZE=25
export NODE_MEMORY_SIZE=8192
export NODE_CPU_COUNT=2
export SSH_USER="clastix"
export SSH_AUTHORIZED_KEY="ssh-rsa AAAAB3N..."
```

The following command will generate a cluster manifest based on the [`capi-kamaji-vsphere-template.yaml`](https://raw.githubusercontent.com/clastix/cluster-api-control-plane-provider-kamaji/master/templates/capi-kamaji-vsphere-template.yaml) template file:

```bash
clusterctl generate cluster $CLUSTER_NAME \
    --from capi-kamaji-vsphere-template.yaml \
    > capi-kamaji-vsphere-cluster.yaml
```

If you want to use DHCP instead of IPAM, use the [`capi-kamaji-vsphere-dhcp-template.yaml`](https://raw.githubusercontent.com/clastix/cluster-api-control-plane-provider-kamaji/master/templates/capi-kamaji-vsphere-dhcp-template.yaml) template file:

```bash
clusterctl generate cluster $CLUSTER_NAME \
    --from capi-kamaji-vsphere-dhcp-template.yaml \
    > capi-kamaji-vsphere-cluster.yaml
```

### Additional cloud-init configuration

Cluster API requires to use templates for the machines, which are based on `cloud-init`. You can add additional `cloud-init` configuration to further customize the worker nodes by including an additional `cloud-init` file in the `KubeadmConfigTemplate`:

```yaml
kind: KubeadmConfigTemplate
apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
metadata:
  name: ${CLUSTER_NAME}-md-0
spec:
  template:
    spec:
      files:
      - path: "/etc/cloud/cloud.cfg.d/99-custom.cfg"
        content: "${CLOUD_INIT_CONFIG:-}"
        owner: "root:root"
        permissions: "0644"
```

You can then set the `CLOUD_INIT_CONFIG` environment variable to include the additional configuration:

```bash
export CLOUD_INIT_CONFIG="#cloud-config package_update: true packages: - net-tools"
```

and include it in the `clusterctl generate cluster` command:

```bash
clusterctl generate cluster $CLUSTER_NAME \
    --from capi-kamaji-vsphere-template.yaml \
    > capi-kamaji-vsphere-cluster.yaml
```

### Apply the Cluster Manifest

Apply the generated cluster manifest to create the tenant cluster:

```bash
kubectl apply -f capi-kamaji-vsphere-cluster.yaml
```

You can check the status of the cluster deployment with `clusterctl`:

```bash
clusterctl describe cluster $CLUSTER_NAME
```

You can check the status of the tenant cluster with `kubectl`:

```bash
kubectl get clusters -n default
```

and related tenant control plane created on the Kamaji Management Cluster:

```bash
kubectl get tcp -n default
```

## Access the Tenant Cluster

To access the tenant cluster, you can estract the `kubeconfig` file from the Kamaji Management Cluster:

```bash
clusterctl get kubeconfig $CLUSTER_NAME \
    > ~/.kube/$CLUSTER_NAME.kubeconfig
```

and use it to access the tenant cluster:

```bash
export KUBECONFIG=~/.kube/$CLUSTER_NAME.kubeconfig
kubectl cluster-info
```

## Cloud Controller Manager

The template file `capi-kamaji-vsphere-template.yaml` includes the external [Cloud Controller Manager (CCM)](https://github.com/kubernetes/cloud-provider-vsphere) configuration for vSphere. The CCM is a Kubernetes controller that manages the cloud provider's resources.

Usually, the CCM is deployed on control plane nodes, but in Kamaji there are no nodes for Control Plane, so the CCM is deployed on the worker nodes as daemonset.

As alternative, you can deploy the CCM as part of the Hosted Control Plane on the Management Cluster. To do so, the template file [`capi-kamaji-vsphere-template-ccm.yaml`](https://raw.githubusercontent.com/clastix/cluster-api-control-plane-provider-kamaji/master/templates/capi-kamaji-vsphere-template-ccm.yaml) includes the configuration for the CCM as part of the Kamaji Control Plane. This approach provides security benefits by isolating vSphere credentials from tenant users while maintaining full Cluster API integration.

The following command will generate a cluster manifest with the CCM installed on the Management Cluster:

```bash
clusterctl generate cluster $CLUSTER_NAME \
    --from capi-kamaji-vsphere-template-ccm.yaml \
    > capi-kamaji-vsphere-cluster.yaml
```

Apply the generated cluster manifest to create the tenant cluster:

```bash
kubectl apply -f capi-kamaji-vsphere-cluster.yaml
```

## vSphere CSI Driver

The template file `capi-kamaji-vsphere-template-csi.yaml` includes the [vSphere CSI Driver](https://github.com/kubernetes-sigs/vsphere-csi-driver) configuration for vSphere. The vSphere CSI Driver is a Container Storage Interface (CSI) driver that provides a way to use vSphere storage with Kubernetes.

This template file introduces a *"split configuration"* for the vSphere CSI Driver, with the CSI driver deployed on the worker nodes as daemonset and the CSI Controller Manager deployed on the Management Cluster as part of the Hosted Control Plane. In this way, no vSphere credentials are required on the tenant cluster.

This spit architecture enables:

* Tenant isolation from vSphere credentials
* Simplified networking requirements
* Centralized controller management

The template file also include a default storage class for the vSphere CSI Driver.

Set the environment variables to match your storage configuration.

For example:

```bash
# Storage Configuration
export CSI_INSECURE="false"
export CSI_LOG_LEVEL="PRODUCTION" # or "DEVELOPMENT"
export CSI_STORAGE_CLASS_NAME="vsphere-csi"
```

The following command will generate a cluster manifest with split configuration for the vSphere CSI Driver:

```bash
clusterctl generate cluster $CLUSTER_NAME \
    --from capi-kamaji-vsphere-template-csi.yaml \
    > capi-kamaji-vsphere-cluster.yaml
```

Apply the generated cluster manifest to create the tenant cluster:

```bash
kubectl apply -f capi-kamaji-vsphere-cluster.yaml
```

## Delete the Tenant Cluster

For cluster deletion, use the following command:

```bash
kubectl delete cluster sample
```

Always use `kubectl delete cluster $CLUSTER_NAME` to delete the tenant cluster. Using `kubectl delete -f capi-kamaji-vsphere-cluster.yaml` may lead to orphaned resources in some scenarios, as this method doesn't always respect ownership references between resources that were created after the initial deployment.

## Install the Tenant Cluster as Helm Release

Another option to create a Tenant Cluster is to use the Helm Chart [cluster-api-kamaji-vsphere](https://github.com/clastix/cluster-api-kamaji-vsphere).

!!! warning "Advanced Usage"
    This Helm Chart provides several additional configuration options to customize the Tenant Cluster. Please refer to its documentation for more information. Make sure you get comfortable with the Cluster API concepts and Kamaji before to attempt to use it.

Create a Tenant Cluster as Helm Release:

```bash
helm repo add clastix https://clastix.github.io/cluster-api-kamaji-vsphere
helm repo update
helm install sample clastix/cluster-api-kamaji-vsphere \
    --set cluster.name=sample \
    --namespace default \
    --values my-values.yaml
```

where `my-values.yaml` is a file containing the configuration values for the Tenant Cluster.
