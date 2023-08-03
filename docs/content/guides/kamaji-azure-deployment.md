# Setup Kamaji on Azure
This guide will lead you through the process of creating a working Kamaji setup on on MS Azure.

!!! warning ""
    The material here is relatively dense. We strongly encourage you to dedicate time to walk through these instructions, with a mind to learning. We do NOT provide any "one-click" deployment here. However, once you've understood the components involved it is encouraged that you build suitable, auditable GitOps deployment processes around your final infrastructure.

The guide requires:

- a bootstrap machine
- a Kubernetes cluster to run the Admin and Tenant Control Planes
- an arbitrary number of machines to host `Tenant`s' workloads

## Summary

  * [Prepare the bootstrap workspace](#prepare-the-bootstrap-workspace)
  * [Access Admin cluster](#access-admin-cluster)
  * [Install Cert Manager](#install-cert-manager)
  * [Install Kamaji controller](#install-kamaji-controller)
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


## Access Admin cluster
In Kamaji, an Admin Cluster is a regular Kubernetes cluster which hosts zero to many Tenant Cluster Control Planes. The admin cluster acts as management cluster for all the Tenant clusters and implements Monitoring, Logging, and Governance of all the Kamaji setup, including all Tenant clusters. For this guide, we're going to use an instance of Azure Kubernetes Service (AKS) as Admin Cluster.

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

## Install Cert Manager

Kamaji takes advantage of the [dynamic admission control](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/), such as validating and mutating webhook configurations. These webhooks are secured by a TLS communication, and the certificates are managed by [`cert-manager`](https://cert-manager.io/), making it a prerequisite that must be installed:

```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install \
  cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version v1.11.0 \
  --set installCRDs=true
```

## Install Kamaji Controller

Installing Kamaji via Helm charts is the preferred way. The Kamaji controller needs to access a Datastore in order to save data of the tenants' clusters. The Kamaji Helm Chart provides the installation of a basic unmanaged `etcd` as datastore, out of box. 

Install Kamaji with `helm` using an unmanaged `etcd` as default datastore:

```bash
helm repo add clastix https://clastix.github.io/charts
helm repo update
helm install kamaji clastix/kamaji -n kamaji-system --create-namespace
```

!!! note "A managed datastore is highly recommended in production"
     The [kamaji-etcd](https://github.com/clastix/kamaji-etcd) project provides the code to setup a multi-tenant `etcd` running as StatefulSet made of three replicas. Optionally, Kamaji offers support for a more robust storage system, as `MySQL` or `PostgreSQL` compatible database, thanks to the native [kine](https://github.com/k3s-io/kine) integration.

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

### Prepare worker nodes to join

Currently, Kamaji does not provide any helper for creation of tenant worker nodes.
You should get a set of machines from your infrastructure provider, turn them into worker nodes, and then join to the tenant control plane with the `kubeadm`. 

Kamaji is sticking to the [Cluster Management API](https://github.com/kubernetes-sigs/cluster-api) project contracts by providing a `ControlPlane` provider.
An Azure-based cluster is not yet available: the available road-map is available on the [official repository](https://github.com/clastix/cluster-api-control-plane-provider-kamaji).

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
   --custom-data ./tenant-cloudinit.yaml \
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

### Join worker nodes
The current approach for joining nodes is to use `kubeadm` and therefore, we will create a bootstrap token to perform the action. In order to facilitate the step, we will store the entire command of joining in a variable:

```bash
TENANT_ADDR=$(kubectl -n ${TENANT_NAMESPACE} get svc ${TENANT_NAME} -o json | jq -r ."spec.loadBalancerIP")
JOIN_CMD=$(echo "sudo kubeadm join ${TENANT_ADDR}:6443 ")$(kubeadm --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig token create --print-join-command |cut -d" " -f4-)
```

A bash loop will be used to join all the available nodes.

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

Apply to the tenant cluster:

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