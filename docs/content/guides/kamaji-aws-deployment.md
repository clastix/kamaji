# Setup Kamaji on aws
This guide will lead you through the process of creating a working Kamaji setup on on AWS.

The guide requires:

- a bootstrap machine
- a Kubernetes cluster (EKS) to run the Admin and Tenant Control Planes
- an arbitrary number of machines to host `Tenant`s' workloads

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
- [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
- [eksctl](https://eksctl.io/installation/)

Make sure you have a valid AWS Account, and login to AWS:

> The easiest way to get started with AWS is to create [access keys](https://docs.aws.amazon.com/cli/v1/userguide/cli-authentication-user.html#cli-authentication-user-configure.title) associated to your account

```bash
aws configure
```


## Create Management cluster 

In Kamaji, a Management Cluster is a regular Kubernetes cluster which hosts zero to many Tenant Cluster Control Planes. The Management Cluster acts as cockpit for all the Tenant clusters and implements Monitoring, Logging, and Governance of all the Kamaji setup, including all Tenant Clusters. For this guide, we're going to use an instance of AWS Kubernetes Service (EKS) as Management Cluster.

Throughout the following instructions, shell variables are used to indicate values that you should adjust to your own AWS environment:

### Create networks

In this section, we will create the required VPC and the associated subnets that will host the EKS cluster. We will also create the EIP (Elastic IPs) that will be used as IPs for tenant cluster

```bash
source kamaji-AWS.env
# create vpc
aws ec2 create-vpc --cidr-block $KAMAJI_VPC_CIDR --region $KAMAJI_REGION 
# retreive subnet
export KAMAJI_VPC_ID=$(aws ec2 describe-vpcs --filters "Name=cidr-block,Values=$KAMAJI_VPC_CIDR" --query "Vpcs[0].VpcId" --output text)
# create subnets
aws ec2 create-subnet --vpc-id $KAMAJI_VPC_ID --cidr-block $KAMAJI_SUBNET1_ADDRESS --availability-zone ${KAMAJI_REGION}a
aws ec2 create-subnet --vpc-id $KAMAJI_VPC_ID --cidr-block $KAMAJI_SUBNET2_ADDRESS --availability-zone ${KAMAJI_REGION}b
# retreive subnets
export KAMAJI_SUBNET1_ID=$(aws ec2 describe-subnets --filter "Name=vpc-id,Values=$KAMAJI_VPC_ID" --filter "Name=cidr-block,Values=$KAMAJI_SUBNET1_ADDRESS"  --query "Subnets[0].SubnetId" --output text)
export KAMAJI_SUBNET2_ID=$(aws ec2 describe-subnets --filter "Name=vpc-id,Values=$KAMAJI_VPC_ID" --filter "Name=cidr-block,Values=$KAMAJI_SUBNET2_ADDRESS"  --query "Subnets[0].SubnetId" --output text)


export IGW_ID=$(aws ec2 create-internet-gateway --query "InternetGateway.InternetGatewayId" --output text)
aws ec2 attach-internet-gateway --vpc-id $KAMAJI_VPC_ID --internet-gateway-id $IGW_ID

# create nat gateway and attach it to the VPC

export EIP_ALLOCATION_ID=$(aws ec2 allocate-address --query 'AllocationId' --output text)

NAT_GATEWAY_ID=$(aws ec2 create-nat-gateway \
  --subnet-id $KAMAJI_SUBNET1_ID \
  --allocation-id $EIP_ALLOCATION_ID \
  --query 'NatGateway.NatGatewayId' \
  --output text)

aws ec2 wait nat-gateway-available --nat-gateway-ids $NAT_GATEWAY_ID

PRIVATE_ROUTE_TABLE_ID=$(aws ec2 describe-route-tables \
   --filters "Name=vpc-id,Values=$KAMAJI_VPC_ID" \
  --query "RouteTables[*].RouteTableId" \
  --output text)

aws ec2 create-route \
  --route-table-id $PRIVATE_ROUTE_TABLE_ID \
  --destination-cidr-block 0.0.0.0/0 \
  --nat-gateway-id $NAT_GATEWAY_ID

  

```
### create EKS cluster
Once the cluster formation succeeds, get credentials to access the cluster as admin

```bash
cat >eks-cluster.yaml <<EOF
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig

metadata:
  name: ${KAMAJI_CLUSTER}
  region: ${KAMAJI_REGION}

vpc:
  subnets:
    private:
      ${KAMAJI_REGION}a: { id: $KAMAJI_SUBNET1_ID }
      ${KAMAJI_REGION}b: { id: $KAMAJI_SUBNET2_ID }

  clusterEndpoints:
    privateAccess: true
    publicAccess: true

managedNodeGroups:
  - name: ${KAMAJI_NODE_NG}
    labels: { role: workers }
    instanceType: ${KAMAJI_NODE_TYPE}
    desiredCapacity: 1
    privateNetworking: true
    iam:
      withAddonPolicies:
        certManager: true
        ebs: true
        externalDNS: true
EOF

eks create cluster -f eks-cluster.yaml

```
### Access to the management cluster

And check you can access:

```bash
aws eks update-kubeconfig --region ${KAMAJI_REGION} --name ${KAMAJI_CLUSTER}
kubectl cluster-info
# make ebs as a default storage class
kubectl patch storageclass standard -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}'

```
### Add route 53 domain 
In order to easily access to tenant clusters , it is recommended to create a route53 domain or use an existing one if exists

```bash
# for within VPC
aws route53 create-hosted-zone --name "$TENANT_DOMAIN" --caller-reference $(date +%s) --vpc "VPCRegion=$KAMAJI_REGION,VPCId=$KAMAJI_VPC_ID"

```
## Install Kamaji

Follow the [Getting Started](../getting-started.md) to install Cert Manager and the Kamaji Controller.

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

### Install externalDNS

Setting externalDNS allows to update your DNS records dynamically from an annotation that you add in the service within EKS. Run the following commands to install externalDNS helm chart:


```bash

helm repo add external-dns https://kubernetes-sigs.github.io/external-dns/
helm repo update
helm install external-dns external-dns/external-dns \
  --namespace external-dns \
  --create-namespace \
  --version 1.15.1
```
## Install Kamaji Controller

Installing Kamaji via Helm charts is the preferred way. Run the following commands to install a stable release of Kamaji:

```bash
helm repo add clastix https://clastix.github.io/charts
helm repo update
helm install kamaji clastix/kamaji -n kamaji-system --create-namespace
```

## Create Tenant Cluster

### Tenant Control Plane
With Kamaji on EKS, the tenant control plane is accessible:

- from management cluster through a `ClusterIP` service
- from tenant worker nodes through an internal loadbalancer
- from tenant admin user through an external loadbalancer responding to `https://${TENANT_NAME}.${TENANT_NAME}.${TENANT_DOMAIN}:443`

Create a tenant control plane of example:

```yaml
cat > ${TENANT_NAMESPACE}-${TENANT_NAME}-2.yaml <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${TENANT_NAMESPACE}
---
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
      replicas: 1
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
            service.beta.kubernetes.io/aws-load-balancer-backend-protocol: tcp
            service.beta.kubernetes.io/aws-load-balancer-scheme: internet-facing
            service.beta.kubernetes.io/aws-load-balancer-subnets: PUBLIC_SUBNET
            service.beta.kubernetes.io/aws-load-balancer-type: nlb
            external-dns.alpha.kubernetes.io/hostname: ${TENANT_NAME}.${TENANT_DOMAIN}
      serviceType: LoadBalancer
  kubernetes:
    version: ${TENANT_VERSION}
    kubelet:
      cgroupfs: systemd
    admissionControllers:
      - ResourceQuota
      - LimitRanger
  networkProfile:
    address: <PUBLIC IP>
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
EOF

kubectl -n ${TENANT_NAMESPACE} apply -f ${TENANT_NAMESPACE}-${TENANT_NAME}.yaml
```

Make sure:

- the following annotation: `external-dns.alpha.kubernetes.io/hostname` is set to create the dns record. It tells AWS to expose the Tenant Control Plane with public domain name: `${TENANT_NAME}.${TENANT_DOMAIN}`.

> Since AWS load Balancer does not support setting LoadBalancerIP, you will get the folowing warning on the service created for the control plane tenant `Error syncing load balancer: failed to ensure load balancer: LoadBalancerIP cannot be specified for AWS ELB`. you can ignore it for now.

### Working with Tenant Control Plane

Check the access to the Tenant Control Plane:

> if the domain you used is a private route53 domain make sure to map the public IP of the LB to ${TENANT_NAME}.${TENANT_DOMAIN} in your `/etc/hosts`. otherwise kubectl will fail checking ssl certificates

```bash
curl -k https://${TENANT_NAME}.${TENANT_DOMAIN}/healthz
curl -k https://${TENANT_NAME}.${TENANT_DOMAIN}/version
```

Let's retrieve the `kubeconfig` in order to work with it:

```bash
kubectl get secrets -n ${TENANT_NAMESPACE} ${TENANT_NAME}-admin-kubeconfig -o json \
  | jq -r '.data["admin.conf"]' \
  | base64 --decode \
  > ${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig

kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig config \
  set-cluster ${TENANT_NAME} \
  --server https://${TENANT_NAME}.${TENANT_DOMAIN}
```

and let's check it out:

```
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get svc

NAME         TYPE        CLUSTER-IP   EXTERNAL-IP   PORT(S)   AGE
kubernetes   ClusterIP   10.96.0.1    <none>        443/TCP   38h
```

Check out how the Tenant Control Plane advertises itself:

```
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get ep

NAME         ENDPOINTS            AGE
kubernetes   172.20.251.60:6443   38h
```

## Join worker nodes

The Tenant Control Plane is made of pods running in the Kamaji Management Cluster. At this point, the Tenant Cluster has no worker nodes. So, the next step is to join some worker nodes to the Tenant Control Plane.

Kamaji does not provide any helper for creation of tenant worker nodes, instead it leverages the [Cluster Management API](https://github.com/kubernetes-sigs/cluster-api). This allows you to create the Tenant Clusters, including worker nodes, in a completely declarative way. Currently, a Cluster API `ControlPlane` provider for AWS is available: check the [official documentation](https://github.com/clastix/cluster-api-control-plane-provider-kamaji/blob/master/docs/providers-aws.md). 

An alternative approach to create and join worker nodes in AWS is to manually create the VMs, turn them into Kubernetes worker nodes and then join through the `kubeadm` command.

### Create the kubeadm join command

Run the following command to get the `kubeadm` join command that will be used on the worker tenant nodes:
```bash
TENANT_ADDR=$(kubectl -n ${TENANT_NAMESPACE} get svc ${TENANT_NAME} -o json | jq -r ."spec.loadBalancerIP")
JOIN_CMD=$(echo "sudo kubeadm join ${TENANT_ADDR}:6443 ")$(kubeadm --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig token create --ttl 0 --print-join-command |cut -d" " -f4-)
```

> setting `--ttl=0` on the `kubeadm token create` will guarantee that the token will never expires and can be used every time.

### create tenant worker nodes ASG

Create an AWS autoscaling group to host tenant worker nodes:

```bash
aws ec2 create-subnet --vpc-id $KAMAJI_VPC_ID --cidr-block $TENANT_SUBNET_ADDRESS --availability-zone ${KAMAJI_REGION}a

export TENANT_SUBNET_ID=$(aws ec2 describe-subnets --filter "Name=vpc-id,Values=$KAMAJI_VPC_ID" --filter "Name=cidr-block,Values=$TENANT_SUBNET_ADDRESS"  --query "Subnets[0].SubnetId" --output text)

USER_DATA=$(cat <<EOF
sudo apt-get update
sudo apt-get install -y apt-transport-https ca-certificates software-properties-common curl gpg
mkdir -p -m 755 /etc/apt/keyrings
curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.30/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.30/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list
curl -fsSL https://pkgs.k8s.io/addons:/cri-o:/stable:/v1.30/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/cri-o-apt-keyring.gpg

echo "deb [signed-by=/etc/apt/keyrings/cri-o-apt-keyring.gpg] https://pkgs.k8s.io/addons:/cri-o:/stable:/v1.30/deb/ /" | sudo tee /etc/apt/sources.list.d/cri-o.list
sudo apt-get update
sudo apt-get install -y cri-o kubelet kubeadm kubectl
sudo apt-mark hold kubelet kubeadm kubectl
sudo systemctl start crio.service
sudo swapoff -a
sudo modprobe br_netfilter
sudo sysctl -w net.ipv4.ip_forward=1
$JOIN_CMD
EOF
)
USER_DATA_ENCODED=$(echo "$USER_DATA" | base64)

LAUNCH_TEMPLATE_ID=$(aws ec2 create-launch-template \
  --launch-template-name "$LAUNCH_TEMPLATE_NAME" \
  --version-description "Initial version" \
  --launch-template-data "{
    \"ImageId\": \"$UBUNTU_AMI_ID\",
    \"InstanceType\": \"$TENANT_VM_SIZE\",
    \"SecurityGroupIds\": [\"$SECURITY_GROUP_ID\"],
    \"UserData\": \"$USER_DATA_ENCODED\"
  }" \
  --query 'LaunchTemplate.LaunchTemplateId' --output text)

  aws autoscaling create-auto-scaling-group \
  --auto-scaling-group-name "$TENANT_ASG_NAME" \
  --launch-template "LaunchTemplateId=$LAUNCH_TEMPLATE_ID,Version=1" \
  --min-size $TENANT_ASG_MIN_SIZE \
  --max-size $TENANT_ASG_MAX_SIZE \
  --desired-capacity $TENANT_ASG_DESIRED_CAPACITY \
  --vpc-zone-identifier "$TENANT_SUBNET_ID" \

```

> Note: we're using the `userdata` in order to bootstrap the worker nodes. You can follow the [documentation](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/) for manual bootstrapping

Checking the nodes:

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get nodes

NAME           STATUS     ROLES    AGE   VERSION
ip-10-0-1-49   NotReady   <none>   56m   v1.30.9
```

The cluster needs a [CNI](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/network-plugins/) plugin to get the nodes ready. In this guide, we are going to install [calico](https://projectcalico.docs.tigera.io/about/about-calico), but feel free to use one of your taste.

Download the latest stable Calico manifest:

```bash
curl https://raw.githubusercontent.com/projectcalico/calico/v3.24.1/manifests/calico.yaml -O
```

As per [documentation](https://projectcalico.docs.tigera.io/reference/public-cloud/AWS), Calico in VXLAN mode is supported on AWS while IPIP packets are blocked by the AWS network fabric. Make sure you edit the manifest above and set the following variables:

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
ip-10-0-1-49       Ready    <none>   56m     v1.30.9
```

## Cleanup
To get rid of the Kamaji infrastructure, remove the RESOURCE_GROUP:

```
TODO
```

That's all folks!