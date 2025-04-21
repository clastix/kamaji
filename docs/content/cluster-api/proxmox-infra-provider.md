# Proxmox VE Infra Provider

Use the Cluster API [Proxmox VE Infra Provider ](https://github.com/ionos-cloud/cluster-api-provider-proxmox) to create a fully functional Kubernetes cluster with the Cluster API [Kamaji Control Plane Provider](https://github.com/clastix/cluster-api-control-plane-provider-kamaji).

The Proxmox Cluster API implementation is developed and maintained by [IONOS Cloud](https://github.com/ionos-cloud).

## Proxmox VE Requirements

A Template VM built using the [Proxmox Builder](https://image-builder.sigs.k8s.io/capi/providers/proxmox) is necessary to create the cluster machines.

## Install the Proxmox VE Infrastructure Provider

To use the Proxmox Cluster API provider, you must connect and authenticate to a Proxmox VE system.

```bash
# The Proxmox VE host
export PROXMOX_URL: "https://pve.example:8006"

# The Proxmox VE TokenID for authentication
export PROXMOX_TOKEN: "clastix@pam!capi"

# The secret associated with the TokenID
export PROXMOX_SECRET: "REDACTED"
```

Install the Infrastructure Provider:

```bash
clusterctl init --infrastructure proxmox
```

## Install the IPAM Provider

To assign IP addresses to nodes, you can use the in-cluster [IPAM provider](https://github.com/kubernetes-sigs/cluster-api-ipam-provider-in-cluster). To do so, initialize the Management Cluster with the `--ipam in-cluster` flag:

```bash
clusterctl init --ipam in-cluster
```

## Create a Tenant Cluster

Once all controllers are running in the management cluster, you can generate and apply the cluster manifests for the tenant cluster you want to provision.

### Generate the Cluster Manifest using the template

Use `clusterctl` to generate a tenant cluster manifest for your Proxmox VE. Set the following environment variables to match the workload cluster configuration:

```bash
# Cluster Configuration
export CLUSTER_NAME="sample"
export CLUSTER_NAMESPACE="default"
export CONTROL_PLANE_REPLICAS=2
export KUBERNETES_VERSION="v1.31.4"
export CLUSTER_DATASTORE="default"
```

Set the following environment variables to configure the workload cluster network:

```bash
# Networking Configuration
export IP_RANGE='["192.168.100.100-192.168.100.200"]'
export IP_PREFIX=24
export GATEWAY="192.168.100.1"
export DNS_SERVERS='["8.8.8.8"]'
export NETWORK_BRIDGE="vmbr0"
export NETWORK_MODEL="virtio"
```

Set the following environment variables to configure the workload machines:

```bash
# Node Configuration
export SSH_USER="clastix"
export SSH_AUTHORIZED_KEY="ssh-rsa AAAAB3Nz ..."
export NODE_REPLICAS=2

# Resource Configuration
export SOURCE_NODE="labs"
export TEMPLATE_ID=100
export ALLOWED_NODES='["labs"]'
export MEMORY_MIB=4096
export NUM_CORES=2
export NUM_SOCKETS=2
export BOOT_VOLUME_DEVICE="scsi0"
export BOOT_VOLUME_SIZE=20
export FILE_STORAGE_FORMAT="qcow2"
export STORAGE_NODE="local"
```

Use the following command to generate a cluster manifest based on the [`capi-kamaji-proxmox-template.yaml`](https://raw.githubusercontent.com/clastix/cluster-api-control-plane-provider-kamaji/master/templates/proxmox/capi-kamaji-proxmox-template.yaml) template file:

```bash
clusterctl generate cluster $CLUSTER_NAME \
    --from capi-kamaji-proxmox-template.yaml \
    > capi-kamaji-proxmox-cluster.yaml
```

### Additional cloud-init configuration

Cluster API requires machine templates based on `cloud-init`. You can add additional `cloud-init` configuration to further customize the worker nodes by including an additional `cloud-init` file in the `KubeadmConfigTemplate`:

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
    --from capi-kamaji-proxmox-template.yaml \
    > capi-kamaji-proxmox-cluster.yaml
```

### Apply the Cluster Manifest

Apply the generated cluster manifest to provision the tenant cluster:

```bash
kubectl apply -f capi-kamaji-proxmox-cluster.yaml
```

Check the status of the cluster deployment using `clusterctl`:

```bash
clusterctl describe cluster $CLUSTER_NAME
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

## Delete the Tenant Cluster

For cluster deletion, use the following command:

```bash
kubectl delete cluster $CLUSTER_NAME
```

Always use `kubectl delete cluster $CLUSTER_NAME` to delete the tenant cluster. Using `kubectl delete -f capi-kamaji-proxmox-cluster.yaml` may lead to orphaned resources in some scenarios, as this method doesn't always respect ownership references between resources that were created after the initial deployment.

## Install the Tenant Cluster as Helm Release

Alternatively, you can create a Tenant Cluster using the Helm Chart [cluster-api-kamaji-proxmox](https://github.com/clastix/cluster-api-kamaji-proxmox).

Create a Tenant Cluster as Helm Release:

```bash
helm repo add clastix https://clastix.github.io/cluster-api-kamaji-proxmox
helm repo update
helm install sample clastix/cluster-api-kamaji-proxmox \
    --set cluster.name=sample \
    --namespace default \
    --values my-values.yaml
```

where `my-values.yaml` is a file containing the configuration values for the Tenant Cluster.
