# Kamaji on Kind
This guide will lead you through the process of creating a working Kamaji setup using Kind cluster. The guide requires the following installed on your workstation: `docker`, `kind`, `helm`, and `kubectl`.

!!! warning "Development Only"
    Run Kamaji on kind only for development or learning purposes.
    
    Kamaji is designed to be run on production-grade Kubernetes clusters, such as those provided by cloud providers or on-premises solutions. Kind is not a production-grade Kubernetes cluster, and it is not recommended to run in production environments.

## Summary

  * [Creating Kind Cluster](#creating-kind-cluster)
  * [Installing Cert-Manager](#installing-cert-manager)
  * [Installing MetalLb](#installing-metallb)
  * [Creating IP Address Pool](#creating-ip-address-pool)
  * [Installing Kamaji](#installing-kamaji)
  * [Creating Tenant Control Plane](#creating-tenant-control-plane)


## Creating Kind Cluster

Create a kind cluster.
```
kind create cluster --name kamaji
```

This will take a short while for the kind cluster to be created.

## Installing Cert-Manager

Kamaji has a dependency on Cert Manager, as it uses dynamic admission control, validating and mutating webhook configurations which are secured by a TLS communication, these certificates are managed by `cert-manager`. Hence, it needs to be added. 

Add the Bitnami Repo to the Helm Manager.

```
helm repo add jetstack https://charts.jetstack.io
helm repo update
```

Install Cert Manager using Helm

```
helm install \
  cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --set installCRDs=true
```

This will install cert-manager to the cluster. You can watch the progress of the installation on the cluster using the command

```
kubectl get pods -Aw
```

## Installing MetalLb 

MetalLB is used in order to dynamically assign IP addresses to the components, and also define custom IP Address Pools. Install MetalLb using the `kubectl` command for apply the manifest:

```
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.15.3/config/manifests/metallb-native.yaml
```

This will install MetalLb onto the cluster with all the necessary resources.

## Creating IP Address Pool

Extract the Gateway IP of the network Kind is running on.

```
# docker
GW_IP=$(docker network inspect -f '{{range .IPAM.Config}}{{.Gateway}}{{end}}' kind)

# podman
GW_IP=$(podman network inspect kind --format '{{(index .Subnets 1).Gateway}}')
```

Modify the IP Address, and create the resource to be added to the cluster to create the IP Address Pool

```
NET_IP=$(echo ${GW_IP} | sed -E 's|^([0-9]+\.[0-9]+)\..*$|\1|g')
cat << EOF | sed -E "s|172.19|${NET_IP}|g" | kubectl apply -f -
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: kind-ip-pool
  namespace: metallb-system
spec:
  addresses:
  - 172.19.255.200-172.19.255.250
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: emtpy
  namespace: metallb-system
EOF
```

## Installing Kamaji
- Add the Clastix Repo to the Helm Manager.

```
helm repo add clastix https://clastix.github.io/charts
```

- Install Kamaji with Helm

```
helm upgrade --install kamaji clastix/kamaji \
  --namespace kamaji-system \
  --create-namespace \
  --set 'resources=null' \
  --version 0.0.0+latest
```

- Watch the progress of the deployments

```
kubectl get pods -Aw 
```

- Verify by first checking Kamaji CRDs

```
kubectl get crds | grep -i kamaji
```

!!! Info "CSI Drivers"
    Kamaji requires a __storage provider__ installed on the management cluster. Kind by default provides `local-path-provisioner`, but one can have any other CSI Drivers.

## Creating Tenant Control Plane

- Create a Tenant Control Plane using the command

```
kubectl apply -f https://raw.githubusercontent.com/clastix/kamaji/master/config/samples/kamaji_v1alpha1_tenantcontrolplane.yaml
```

- Watch the progress of the Tenant Control Plane by

```
kubectl get tcp -w
```

- You can attempt to get the details of the control plane by downloading the `kubeconfig` file

```
# Set the SECRET as KUBECONFIG column listed in the tcp output.
SECRET=""
kubectl get secret $SECRET -o jsonpath='{.data.admin\.conf}'|base64 -d > /tmp/kamaji.conf
```

- (options) if you run kind in some specific systems with `docker bridge network`, eg macOS, you may need to access the `kind` container, and perform the `kubectl` actions:

```
docker exec -it $(docker container list | grep kamaji-control-plane | awk '{print $1}') bash
```

- Export the `kubeconfig` file to the environment variable `KUBECONFIG`

```
export KUBECONFIG=/tmp/kamaji.conf
```

- Notice that the `kubectl` version changes, and there are no nodes now.

```
kubectl version
kubectl get nodes
```

A Video Tutorial of the [demonstration](https://www.youtube.com/watch?v=hDTvnOyUmo4&t=577s) can also be viewed.
