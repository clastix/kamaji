# Kamaji on Azure
This guide will lead you through the process of creating a working Kamaji setup on on MS Azure.

The guide requires:

- a bootstrap machine
- a Kubernetes cluster (AKS) to run the Management and Tenant Control Planes
- an arbitrary number of machines to host Tenant workloads.

## Summary

  * [Prepare the bootstrap workspace](#prepare-the-bootstrap-workspace)
  * [Access Management Cluster](#access-management-cluster)
  * [Install Kamaji](#install-kamaji)
  * [Create Tenant Cluster](#create-tenant-cluster)
  * [Cleanup](#cleanup)

## Prepare the bootstrap workspace
On the bootstrap machine, clone the repo and prepare the workspace directory:

```bash
git clone https://github.com/clastix/kamaji
cd kamaji/deploy
```

We assume you have installed on the bootstrap machine:

- [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
- [kubeadm](https://kubernetes.io/docs/tasks/tools/#kubeadm)
- [helm](https://helm.sh/docs/intro/install/)
- [jq](https://stedolan.github.io/jq/)
- [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli)

Make sure you have a valid Azure subscription, and login to Azure:

```bash
az account set --subscription "MySubscription"
az login
```


## Access Management Cluster
In Kamaji, a Management Cluster is a regular Kubernetes cluster which hosts zero to many Tenant Cluster Control Planes. The Management Cluster acts as cockpit for all the Tenant clusters and implements Monitoring, Logging, and Governance of all the Kamaji setup, including all Tenant Clusters. For this guide, we're going to use an instance of Azure Kubernetes Service (AKS) as Management Cluster.

Throughout the following instructions, shell variables are used to indicate values that you should adjust to your own Azure environment:

```bash
source kamaji-azure.env

az group create \
  --name $KAMAJI_RG \
  --location $KAMAJI_REGION

az network vnet create \
  --resource-group $KAMAJI_RG \
  --name $KAMAJI_VNET_NAME \
  --location $KAMAJI_REGION \
  --address-prefix $KAMAJI_VNET_ADDRESS

az network vnet subnet create \
  --resource-group $KAMAJI_RG \
  --name $KAMAJI_SUBNET_NAME \
  --vnet-name $KAMAJI_VNET_NAME \
  --address-prefixes $KAMAJI_SUBNET_ADDRESS

KAMAJI_SUBNET_ID=$(az network vnet subnet show \
  --resource-group ${KAMAJI_RG} \
  --vnet-name ${KAMAJI_VNET_NAME} \
  --name ${KAMAJI_SUBNET_NAME} \
  --query id --output tsv)

az aks create \
  --resource-group $KAMAJI_RG \
  --name $KAMAJI_CLUSTER \
  --location $KAMAJI_REGION \
  --vnet-subnet-id $KAMAJI_SUBNET_ID \
  --zones 1 2 3 \
  --node-count 3 \
  --nodepool-name $KAMAJI_CLUSTER
```

Once the cluster formation succedes, get credentials to access the cluster as admin

```bash
az aks get-credentials  \
  --resource-group $KAMAJI_RG \
  --name $KAMAJI_CLUSTER
```

And check you can access:

```bash
kubectl cluster-info
```

## Install Kamaji

Follow the [Getting Started](kamaji-generic.md) to install Cert Manager and the Kamaji Controller.

## Create Tenant Cluster

### Tenant Control Plane
With Kamaji on AKS, the tenant control plane is accessible:

- from tenant worker nodes through an internal loadbalancer
- from tenant admin user through an external loadbalancer responding to `https://${TENANT_NAME}.${TENANT_NAME}.${TENANT_DOMAIN}:443`

Create a tenant control plane of example:

```yaml
cat > ${TENANT_NAMESPACE}-${TENANT_NAME}-tcp.yaml <<EOF
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: ${TENANT_NAME}
  namespace: ${TENANT_NAMESPACE}
  labels:
    tenant.clastix.io: ${TENANT_NAME}
spec:
  dataStore: default
  controlPlane:
    deployment:
      replicas: 3
      additionalMetadata:
        labels:
          tenant.clastix.io: ${TENANT_NAME}
      extraArgs:
        apiServer: []
        controllerManager: []
        scheduler: []
      resources:
        apiServer:
          requests:
            cpu: 250m
            memory: 512Mi
          limits: {}
        controllerManager:
          requests:
            cpu: 125m
            memory: 256Mi
          limits: {}
        scheduler:
          requests:
            cpu: 125m
            memory: 256Mi
          limits: {}
    service:
      additionalMetadata:
        labels:
          tenant.clastix.io: ${TENANT_NAME}
        annotations:
          service.beta.kubernetes.io/azure-load-balancer-internal: "true"
      serviceType: LoadBalancer
  kubernetes:
    version: ${TENANT_VERSION}
    kubelet:
      cgroupfs: systemd
    admissionControllers:
      - ResourceQuota
      - LimitRanger
  networkProfile:
    port: ${TENANT_PORT}
    certSANs:
    - ${TENANT_NAME}.${TENANT_DOMAIN}
    serviceCidr: ${TENANT_SVC_CIDR}
    podCidr: ${TENANT_POD_CIDR}
    dnsServiceIPs:
    - ${TENANT_DNS_SERVICE}
  addons:
    coreDNS: {}
    kubeProxy: {}
    konnectivity:
      server:
        port: ${TENANT_PROXY_PORT}
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits: {}
---
apiVersion: v1
kind: Service
metadata:
  name: ${TENANT_NAME}-public
  namespace: ${TENANT_NAMESPACE}
  annotations:
    service.beta.kubernetes.io/azure-dns-label-name: ${TENANT_NAME}
spec:
  ports:
  - port: 443
    protocol: TCP
    targetPort: ${TENANT_PORT}
  selector:
    kamaji.clastix.io/name: ${TENANT_NAME}
  type: LoadBalancer
EOF

kubectl -n ${TENANT_NAMESPACE} apply -f ${TENANT_NAMESPACE}-${TENANT_NAME}-tcp.yaml
```

Make sure:

- the following annotation: `service.beta.kubernetes.io/azure-load-balancer-internal=true` is set on the `tcp` service. It tells Azure to expose the service within an internal loadbalancer.

- the following annotation: `service.beta.kubernetes.io/azure-dns-label-name=${TENANT_NAME}` is set the public loadbalancer service. It tells Azure to expose the Tenant Control Plane with public domain name: `${TENANT_NAME}.${TENANT_DOMAIN}`.

### Working with Tenant Control Plane

Check the access to the Tenant Control Plane:

```bash
curl -k https://${TENANT_NAME}.${KAMAJI_REGION}.cloudapp.azure.com/healthz
curl -k https://${TENANT_NAME}.${KAMAJI_REGION}.cloudapp.azure.com/version
```

Let's retrieve the `kubeconfig` in order to work with it:

```bash
kubectl get secrets -n ${TENANT_NAMESPACE} ${TENANT_NAME}-admin-kubeconfig -o json \
  | jq -r '.data["admin.conf"]' \
  | base64 --decode \
  > ${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig

kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig config \
  set-cluster ${TENANT_NAME} \
  --server https://${TENANT_NAME}.${KAMAJI_REGION}.cloudapp.azure.com
```

and let's check it out:

```
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get svc

NAMESPACE     NAME         TYPE        CLUSTER-IP   EXTERNAL-IP   PORT(S)     AGE
default       kubernetes   ClusterIP   10.32.0.1    <none>        443/TCP     6m
```

Check out how the Tenant Control Plane advertises itself:

```
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get ep

NAME         ENDPOINTS           AGE
kubernetes   10.240.0.100:6443   57m
```

### Join worker nodes

The Tenant Control Plane is made of pods running in the Kamaji Management Cluster. At this point, the Tenant Cluster has no worker nodes. So, the next step is to join some worker nodes to the Tenant Control Plane.

Kamaji does not provide any helper for creation of tenant worker nodes, instead it leverages the [Cluster Management API](https://github.com/kubernetes-sigs/cluster-api). This allows you to create the Tenant Clusters, including worker nodes, in a completely declarative way. Currently, a Cluster API `ControlPlane` provider for Azure is not yet available: check the road-map on the [official repository](https://github.com/clastix/cluster-api-control-plane-provider-kamaji). 

An alternative approach to create and join worker nodes in Azure is to manually create the VMs, turn them into Kubernetes worker nodes and then join through the `kubeadm` command.

Create an Azure VM Stateful Set to host worker nodes

```bash
az network vnet subnet create \
   --resource-group $KAMAJI_RG \
   --name $TENANT_SUBNET_NAME \
   --vnet-name $KAMAJI_VNET_NAME \
   --address-prefixes $TENANT_SUBNET_ADDRESS

az vmss create \
   --name $TENANT_VMSS \
   --resource-group $KAMAJI_RG \
   --image $TENANT_VM_IMAGE \
   --vnet-name $KAMAJI_VNET_NAME \
   --subnet $TENANT_SUBNET_NAME \
   --computer-name-prefix $TENANT_NAME- \
   --load-balancer "" \
   --instance-count 0

az vmss update \
   --resource-group $KAMAJI_RG \
   --name $TENANT_VMSS \
   --set virtualMachineProfile.networkProfile.networkInterfaceConfigurations[0].enableIPForwarding=true

az vmss scale \
   --resource-group $KAMAJI_RG \
   --name $TENANT_VMSS \
   --new-capacity 3
```

Once all the machines are ready, follow the related [documentation](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/) in order to:

- install `containerd` as container runtime
- install `crictl`, the command line for working with `containerd`
- install `kubectl`, `kubelet`, and `kubeadm` in the desired version

After the installation is complete on all the nodes, store the entire command of joining in a variable:

```bash
TENANT_ADDR=$(kubectl -n ${TENANT_NAMESPACE} get svc ${TENANT_NAME} -o json | jq -r ."spec.loadBalancerIP")
JOIN_CMD=$(echo "sudo kubeadm join ${TENANT_ADDR}:6443 ")$(kubeadm --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig token create --print-join-command |cut -d" " -f4-)
```

Use a loop to log in to and run the join command on each node:

```bash
VMIDS=($(az vmss list-instances \
   --resource-group $KAMAJI_RG \
   --name $TENANT_VMSS \
   --query [].instanceId \
   --output tsv))

for i in ${!VMIDS[@]}; do
  VMID=${VMIDS[$i]}
  az vmss run-command create \
	  --name join-tenant-control-plane \
	  --vmss-name  $TENANT_VMSS \
	  --resource-group $KAMAJI_RG \
	  --instance-id ${VMID} \
	  --script "${JOIN_CMD}"
done
```

Checking the nodes:

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get nodes

NAME               STATUS     ROLES    AGE    VERSION
tenant-00-000000   NotReady   <none>   112s   v1.25.0
tenant-00-000002   NotReady   <none>   92s    v1.25.0
tenant-00-000003   NotReady   <none>   71s    v1.25.0
```

The cluster needs a [CNI](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/network-plugins/) plugin to get the nodes ready. In this guide, we are going to install [calico](https://projectcalico.docs.tigera.io/about/about-calico), but feel free to use one of your taste.

Download the latest stable Calico manifest:

```bash
curl https://raw.githubusercontent.com/projectcalico/calico/v3.24.1/manifests/calico.yaml -O
```

As per [documentation](https://projectcalico.docs.tigera.io/reference/public-cloud/azure), Calico in VXLAN mode is supported on Azure while IPIP packets are blocked by the Azure network fabric. Make sure you edit the manifest above and set the following variables:

- `CLUSTER_TYPE="k8s"`
- `CALICO_IPV4POOL_IPIP="Never"`
- `CALICO_IPV4POOL_VXLAN="Always"`

Apply to the Tenant Cluster:

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig apply -f calico.yaml
```

And after a while, nodes will be ready

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get nodes 

NAME               STATUS   ROLES    AGE     VERSION
tenant-00-000000   Ready    <none>   3m38s   v1.25.0
tenant-00-000002   Ready    <none>   3m18s   v1.25.0
tenant-00-000003   Ready    <none>   2m57s   v1.25.0
```

## Cleanup
To get rid of the Kamaji infrastructure, remove the RESOURCE_GROUP:

```
az group delete --name $KAMAJI_RG --yes --no-wait
```

That's all folks!