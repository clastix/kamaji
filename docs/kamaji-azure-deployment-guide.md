# Setup Kamaji on Azure
This guide will lead you through the process of creating a working Kamaji setup on on MS Azure. It requires:

- one bootstrap local workstation
- an AKS Kubernetes cluster to run the Admin and Tenant Control Planes
- an arbitrary number of Azure virtual machines to host `Tenant`s' workloads

  * [Prepare the bootstrap workspace](#prepare-the-bootstrap-workspace)
  * [Access Admin cluster](#access-admin-cluster)
  * [Install Kamaji controller](#install-kamaji-controller)
  * [Create Tenant Cluster](#create-tenant-cluster)
  * [Cleanup](#cleanup)

## Prepare the bootstrap workspace
This guide is supposed to be run from a remote or local bootstrap machine. First, clone the repo and prepare the workspace directory:

```bash
git clone https://github.com/clastix/kamaji
cd kamaji/deploy
```

We assume you have installed on your workstation:

- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [helm](https://helm.sh/docs/intro/install/)
- [jq](https://stedolan.github.io/jq/)
- [openssl](https://www.openssl.org/)
- [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli)

Make sure you have a valid Azure subscription, and login to Azure:

```bash
az account set --subscription "MySubscription"
az login
```
> Currently, the Kamaji setup, including Admin and Tenant clusters need to be deployed within the same Azure region. Cross-regions deployments are not supported.

## Access Admin cluster
In Kamaji, an Admin Cluster is a regular Kubernetes cluster which hosts zero to many Tenant Cluster Control Planes. The admin cluster acts as management cluster for all the Tenant clusters and implements Monitoring, Logging, and Governance of all the Kamaji setup, including all Tenant clusters. For this guide, we're going to use an instance of Azure Kubernetes Service - AKS as the Admin Cluster.

Throughout the following instructions, shell variables are used to indicate values that you should adjust to your own Azure environment:

```bash
source kamaji-azure.env

az group create \
  --name $KAMAJI_RG \
  --location $KAMAJI_REGION

az aks create \
  --resource-group $KAMAJI_RG \
  --name $KAMAJI_CLUSTER \
  --location $KAMAJI_REGION \
  --zones 1 2 3 \
  --node-count 3 \
  --nodepool-name $KAMAJI_CLUSTER \
  --ssh-key-value @~/.ssh/id_rsa.pub \
  --no-wait
```

Once the cluster formation succedes, get credentials to access the cluster as admin

```
az aks get-credentials  \
  --resource-group $KAMAJI_RG \
  --name $KAMAJI_CLUSTER
```

And check you can access:

```
kubectl cluster-info
```

## Install Kamaji
There are multiple ways to deploy Kamaji, including a [single YAML file](../config/install.yaml) and [Helm Chart](../helm/kamaji).

### Multi-tenant datastore
The Kamaji controller needs to access a multi-tenant datastore in order to save data of the tenants' clusters.
Install a multi-tenant `etcd` in the admin cluster as three replicas StatefulSet with data persistence.
The Helm [Chart](../helm/kamaji/) provides the installation of an internal `etcd`.
However, an externally managed `etcd` is highly recommended.
If you'd like to use an external one, you can specify the overrides by setting the value `etcd.deploy=false`.

Optionally, Kamaji offers the possibility of using a different storage system than `etcd` for the tenants' clusters, like MySQL or PostgreSQL compatible database, thanks to the [kine](https://github.com/k3s-io/kine) integration documented [here](../deploy/kine/README.md).

### Install with Helm Chart
Install with the `helm` in a dedicated namespace of the Admin cluster:

```bash
helm install --create-namespace --namespace kamaji-system kamaji ../helm/kamaji
```

The Kamaji controller and the multi-tenant `etcd` are now running:

```bash
kubectl -n kamaji-system get pods
NAME                      READY   STATUS    RESTARTS       AGE
etcd-0                    1/1     Running   0              120m
etcd-1                    1/1     Running   0              120m
etcd-2                    1/1     Running   0              119m
kamaji-857fcdf599-4fb2p   2/2     Running   0              120m
```

You just turned your AKS cluster into a Kamaji cluster to run multiple Tenant Control Planes.

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
spec:
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
            cpu: 500m
            memory: 512Mi
          limits: {}
        controllerManager:
          requests:
            cpu: 250m
            memory: 256Mi
          limits: {}
        scheduler:
          requests:
            cpu: 250m
            memory: 256Mi
          limits: {} 
    service:
      additionalMetadata:
        labels:
          tenant.clastix.io: ${TENANT_NAME}
        annotations:
          service.beta.kubernetes.io/azure-load-balancer-internal: "true"
      serviceType: LoadBalancer
    ingress:
      enabled: false
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
      proxyPort: ${TENANT_PROXY_PORT}
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
    kamaji.clastix.io/soot: ${TENANT_NAME}
  type: LoadBalancer
EOF

kubectl create namespace ${TENANT_NAMESPACE}
kubectl apply -f ${TENANT_NAMESPACE}-${TENANT_NAME}-tcp.yaml
```

Make sure:

- the following annotation: `service.beta.kubernetes.io/azure-load-balancer-internal=true` is set on the `tcp` service. It tells Azure to expose the service within an internal loadbalancer.

- the following annotation: `service.beta.kubernetes.io/azure-dns-label-name=${TENANT_NAME}` is set the public loadbalancer service. It tells Azure to expose the Tenant Control Plane with domain name: `${TENANT_NAME}.${TENANT_DOMAIN}`.

### Working with Tenant Control Plane

Check the access to the Tenant Control Plane:

```
curl -k https://${TENANT_NAME}.${KAMAJI_REGION}.cloudapp.azure.com/healthz
curl -k https://${TENANT_NAME}.${KAMAJI_REGION}.cloudapp.azure.com/version
```

Let's retrieve the `kubeconfig` in order to work with it:

```
kubectl get secrets -n ${TENANT_NAMESPACE} ${TENANT_NAME}-admin-kubeconfig -o json \
  | jq -r '.data["admin.conf"]' \
  | base64 -d \
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

### Prepare the Infrastructure for the Tenant virtual machines
Kamaji provides Control Plane as a Service, so the tenant user can join his own virtual machines as worker nodes. Each tenant can place his virtual machines in a dedicated Azure virtual network.

Prepare the Tenant infrastructure:

```
az group create \
    --name $TENANT_RG \
    --location $KAMAJI_REGION

az network nsg create \
    --resource-group $TENANT_RG \
    --name $TENANT_NSG

az network nsg rule create \
    --resource-group $TENANT_RG \
    --nsg-name $TENANT_NSG \
    --name $TENANT_NSG-ssh \
    --protocol tcp \
    --priority 1000 \
    --destination-port-range 22 \
    --access allow

az network vnet create \
    --resource-group $TENANT_RG \
    --name $TENANT_VNET_NAME \
    --address-prefix $TENANT_VNET_ADDRESS \
    --subnet-name $TENANT_SUBNET_NAME \
    --subnet-prefix $TENANT_SUBNET_ADDRESS

az network vnet subnet create \
   --resource-group $TENANT_RG \
   --vnet-name $TENANT_VNET_NAME \
   --name $TENANT_SUBNET_NAME \
   --address-prefixes $TENANT_SUBNET_ADDRESS \
   --network-security-group $TENANT_NSG
```

Connection between the Tenant virtual network and the Kamaji AKS virtual network leverages on the [Azure Network Peering](https://docs.microsoft.com/en-us/azure/virtual-network/virtual-network-peering-overview).

Enable the network peering between the Tenant Virtual Network and the Kamaji AKS Virtual Network:

```bash
KAMAJI_VNET_NAME=`az network vnet list -g $KAMAJI_NODE_RG --query [].name --out tsv`
KAMAJI_VNET_ID=`az network vnet list -g $KAMAJI_NODE_RG --query [].id --out tsv`
TENANT_VNET_ID=`az network vnet list -g $TENANT_RG --query [].id --out tsv`

az network vnet peering create \
   --resource-group $TENANT_RG \
   --name $TENANT_NAME-$KAMAJI_CLUSTER \
   --vnet-name $TENANT_VNET_NAME \
   --remote-vnet $KAMAJI_VNET_ID \
   --allow-vnet-access

az network vnet peering create \
   --resource-group $KAMAJI_NODE_RG \
   --name $KAMAJI_CLUSTER-$TENANT_NAME \
   --vnet-name $KAMAJI_VNET_NAME \
   --remote-vnet $TENANT_VNET_ID \
   --allow-vnet-access
```

[Azure Network Security Groups](https://docs.microsoft.com/en-us/azure/virtual-network/network-security-groups-overview) can be used to control the traffic between the Tenant virtual network and the Kamaji AKS virtual network for a stronger isolation. See the required [ports and protocols](https://kubernetes.io/docs/reference/ports-and-protocols/) between Kubernetes control plane and worker nodes. 

### Create the tenant virtual machines
Create an Azure VM Stateful Set to host virtual machines

```
az vmss create \
   --name $TENANT_VMSS \
   --resource-group $TENANT_RG \
   --image $TENANT_VM_IMAGE \
   --public-ip-per-vm \
   --vnet-name $TENANT_VNET_NAME \
   --subnet $TENANT_SUBNET_NAME \
   --ssh-key-value @~/.ssh/id_rsa.pub \
   --computer-name-prefix $TENANT_NAME- \
   --nsg $TENANT_NSG \
   --custom-data ./tenant-cloudinit.yaml \
   --instance-count 0 

az vmss update \
   --resource-group $TENANT_RG \
   --name $TENANT_VMSS \
   --set virtualMachineProfile.networkProfile.networkInterfaceConfigurations[0].enableIPForwarding=true

az vmss scale \
   --resource-group $TENANT_RG \
   --name $TENANT_VMSS \
   --new-capacity 3
```

### Join the tenant virtual machines to the tenant control plane
The current approach for joining nodes is to use the `kubeadm` one therefore, we will create a bootstrap token to perform the action:

```bash
JOIN_CMD=$(echo "sudo kubeadm join ${TENANT_ADDR}:6443 ")$(kubeadm --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig token create --print-join-command |cut -d" " -f4-)
```

A bash loop will be used to join all the available nodes.

```bash
HOSTS=($(az vmss list-instance-public-ips \
    --resource-group $TENANT_RG \
    --name $TENANT_VMSS \
    --query "[].ipAddress" \
    --output tsv))

for i in ${!HOSTS[@]}; do
  HOST=${HOSTS[$i]}
  echo $HOST
  ssh ${USER}@${HOST} -t ${JOIN_CMD};
done
```

Checking the nodes:

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get nodes

NAME               STATUS     ROLES    AGE    VERSION
tenant-00-000000   NotReady   <none>   112s   v1.23.5
tenant-00-000002   NotReady   <none>   92s    v1.23.5
tenant-00-000003   NotReady   <none>   71s    v1.23.5
```

The cluster needs a [CNI](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/network-plugins/) plugin to get the nodes ready. In our case, we are going to install [calico](https://projectcalico.docs.tigera.io/about/about-calico).

```bash
kubectl apply -f calico-cni/calico-crd.yaml --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig
kubectl apply -f calico-cni/calico-azure.yaml --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig
```

And after a while, `kube-system` pods will be running.

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get po -n kube-system 

NAME                                       READY   STATUS    RESTARTS   AGE
calico-kube-controllers-8594699699-dlhbj   1/1     Running   0          3m
calico-node-kxf6n                          1/1     Running   0          3m
calico-node-qtdlw                          1/1     Running   0          3m
coredns-64897985d-2v5lc                    1/1     Running   0          5m
coredns-64897985d-nq276                    1/1     Running   0          5m
kube-proxy-cwdww                           1/1     Running   0          3m
kube-proxy-m48v4                           1/1     Running   0          3m
```

And the nodes will be ready

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get nodes 

NAME               STATUS   ROLES    AGE     VERSION
tenant-00-000000   Ready    <none>   3m38s   v1.23.5
tenant-00-000002   Ready    <none>   3m18s   v1.23.5
tenant-00-000003   Ready    <none>   2m57s   v1.23.5
```

## Cleanup
To get rid of the Tenant infrastructure, remove the RESOURCE_GROUP:

```
az group delete --name $TENANT_RG --yes --no-wait
```

To get rid of the Kamaji infrastructure, remove the RESOURCE_GROUP:

```
az group delete --name $KAMAJI_RG --yes --no-wait
```

That's all folks!