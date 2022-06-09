# Setup Kamaji on Azure

In this section, we're going to setup Kamaji on MS Azure:

- one bootstrap local workstation
- a regular AKS cluster as Kamaji Admin Cluster
- a multi-tenant etcd internal cluster running on AKS
- an arbitrary number of Azure virtual machines hosting `Tenant`s' workloads

## Bootstrap machine
This getting started guide is supposed to be run from a remote or local bootstrap machine.
First, prepare the workspace directory:

```
git clone https://github.com/clastix/kamaji
cd kamaji/deploy
```

1. Follow the instructions in [Prepare the bootstrap workspace](./getting-started-with-kamaji.md#prepare-the-bootstrap-workspace).
2. Install the [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli).
3. Make sure you have a valid Azure subscription
4. Login to Azure:

```bash
az account set --subscription "MySubscription"
az login
```
> Currently, the Kamaji setup, including Admin and Tenant clusters need to be deployed within the same Azure region. Cross-regions deployments are not (yet) validated.

## Setup Admin cluster on AKS
Throughout the instructions, shell variables are used to indicate values that you should adjust to your own Azure environment:

```bash
source kamaji-azure.env
```

> we use the Azure CLI to setup the Kamaji Admin cluster on AKS.

```
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

## Setup internal multi-tenant etcd
Follow the instructions [here](./kamaji-deployment-guide.md#setup-internal-multi-tenant-etcd).

## Install Kamaji controller
Follow the instructions [here](./kamaji-deployment-guide.md#install-kamaji-controller).

## Create Tenant Clusters
To create a Tenant Cluster in Kamaji on AKS, we have to work on both the Kamaji and Azure infrastructure sides.

```
source kamaji-tenant-azure.env
```

### On Kamaji side
With Kamaji on AKS, the tenant control plane is accessible:

- from tenant work nodes through an internal loadbalancer as `https://${TENANT_ADDR}:6443`
- from tenant admin user through an external loadbalancer `https://${TENANT_NAME}.${KAMAJI_REGION}.cloudapp.azure.com:443`

Where `TENANT_ADDR` is the Azure internal IP address assigned to the LoadBalancer service created by Kamaji to expose the Tenant Control Plane endpoint.

#### Create the Tenant Control Plane

Create the manifest for Tenant Control Plane:

```yaml
cat > ${TENANT_NAMESPACE}-${TENANT_NAME}-tcp.yaml <<EOF
---
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: ${TENANT_NAME}
  namespace: ${TENANT_NAMESPACE}
spec:
  controlPlane:
    deployment:
      replicas: 2
      additionalMetadata:
        annotations:
          environment.clastix.io: ${TENANT_NAME}
        labels:
          tenant.clastix.io: ${TENANT_NAME}
          kind.clastix.io: deployment
    service:
      additionalMetadata:
        annotations:
          environment.clastix.io: ${TENANT_NAME}
          service.beta.kubernetes.io/azure-load-balancer-internal: "true"
        labels:
          tenant.clastix.io: ${TENANT_NAME}
          kind.clastix.io: service
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
    port: 6443
    domain: ${KAMAJI_REGION}.cloudapp.azure.com
    serviceCidr: ${TENANT_SVC_CIDR}
    podCidr: ${TENANT_POD_CIDR}
    dnsServiceIPs:
    - ${TENANT_DNS_SERVICE}
  addons:
    coreDNS:
      enabled: true
    kubeProxy:
      enabled: true
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
```

Make sure:

- the `tcp.spec.controlPlane.service.serviceType=LoadBalancer` and the following annotation: `service.beta.kubernetes.io/azure-load-balancer-internal=true` is set. This tells AKS to expose the service within an Azure internal loadbalancer.

- the public loadbalancer service has the following annotation: `service.beta.kubernetes.io/azure-dns-label-name=${TENANT_NAME}` to expose the Tenant Control Plane with domain name: `${TENANT_NAME}.${KAMAJI_REGION}.cloudapp.azure.com`.

Create the Tenant Control Plane

```
kubectl create namespace ${TENANT_NAMESPACE}
kubectl apply -f ${TENANT_NAMESPACE}-${TENANT_NAME}-tcp.yaml
```

And check it out:

```
$ kubectl get tcp
NAME        VERSION   CONTROL-PLANE-ENDPOINT   KUBECONFIG                   PRIVATE   AGE
tenant-00   v1.23.4   10.240.0.100:6443        tenant-00-admin-kubeconfig   true      46m

$ kubectl get svc
NAME               TYPE           CLUSTER-IP     EXTERNAL-IP      PORT(S)          AGE
tenant-00          LoadBalancer   10.0.223.161   10.240.0.100     6443:31902/TCP   46m
tenant-00-public   LoadBalancer   10.0.205.97    20.101.215.149   443:30697/TCP    19h

$ kubectl get deploy
NAME        READY   UP-TO-DATE   AVAILABLE   AGE
tenant-00   2/2     2            2           47m
```

Collect the internal IP address of Azure loadbalancer where the Tenant control Plane is exposed:

```bash
TENANT_ADDR=$(kubectl -n ${TENANT_NAMESPACE} get svc ${TENANT_NAME} -o json | jq -r ."status.loadBalancer.ingress[].ip")
```

#### Working with Tenant Control Plane
Check the access to the Tenant Control Plane:

```
curl -k https://${TENANT_NAME}.${KAMAJI_REGION}.cloudapp.azure.com/healthz
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

Make sure it's `${TENANT_ADDR}:6443`.

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
kubectl get nodes --kubeconfig=${CLUSTER_NAMESPACE}-${CLUSTER_NAME}.kubeconfig

NAME                      STATUS      ROLES    AGE    VERSION
kamaji-tenant-worker-00   NotReady    <none>   1m     v1.23.4
kamaji-tenant-worker-01   NotReady    <none>   1m     v1.23.4
kamaji-tenant-worker-02   NotReady    <none>   1m     v1.23.4
kamaji-tenant-worker-03   NotReady    <none>   1m     v1.23.4
```

The cluster needs a [CNI](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/network-plugins/) plugin to get the nodes ready. In our case, we are going to install [calico](https://projectcalico.docs.tigera.io/about/about-calico).

```bash
kubectl apply -f calico-cni/calico-crd.yaml --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig
kubectl apply -f calico-cni/calico-azure.yaml --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig
```

And after a while, `kube-system` pods will be running.

```bash
kubectl get po -n kube-system --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig

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
kubectl get nodes --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig

NAME                      STATUS      ROLES    AGE    VERSION
kamaji-tenant-worker-01   Ready       <none>   10m    v1.23.4
kamaji-tenant-worker-02   Ready       <none>   10m    v1.23.4
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