# Setup a minimal Kamaji for development

This document explains how to deploy a minimal Kamaji setup on [KinD](https://kind.sigs.k8s.io/) for development scopes. Please refer to the [Kamaji documentation](../../README.md) for understanding all the terms used in this guide, as for example: `admin cluster` and `tenant control plane`.

## Tools

We assume you have installed on your workstation:

- [Docker](https://docs.docker.com/engine/install/)
- [KinD](https://kind.sigs.k8s.io/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [kubeadm](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/)
- [jq](https://stedolan.github.io/jq/)
- [openssl](https://www.openssl.org/)
- [cfssl](https://github.com/cloudflare/cfssl)
- [cfssljson](https://github.com/cloudflare/cfssl)

## Setup Kamaji on KinD

The instance of Kamaji is made of a single node hosting:

- admin control-plane
- admin worker
- multi-tenant etcd cluster

The multi-tenant etcd cluster is deployed as statefulset into the Kamaji node.

Run `make kamaji` to setup Kamaji on KinD.

```bash
cd ./deploy/kind
make kamaji
```

At this moment you will have your KinD up and running and ETCD cluster in multitenant mode. 

### Install Kamaji

```bash
$ kubectl apply -f ../../config/install.yaml
```

### Deploy Tenant Control Plane

Now it is the moment of deploying your first tenant control plane.

```bash
$ kubectl apply -f - <<EOF
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: tenant1
spec:
  controlPlane:
    deployment:
      replicas: 2
      additionalMetadata:
        annotations:
          environment.clastix.io: tenant1
          tier.clastix.io: "0"
        labels:
          tenant.clastix.io: tenant1
          kind.clastix.io: deployment
    service:
      additionalMetadata:
        annotations:
          environment.clastix.io: tenant1
          tier.clastix.io: "0"
        labels:
          tenant.clastix.io: tenant1
          kind.clastix.io: service
      serviceType: NodePort
    ingress:
      enabled: false
  kubernetes:
    version: "v1.23.4"
    kubelet:
      cgroupfs: cgroupfs
    admissionControllers:
    - LimitRanger
    - ResourceQuota
  networkProfile:
    address: "172.18.0.2"
    port: 31443
    domain: "clastix.labs"
    serviceCidr: "10.96.0.0/16"
    podCidr: "10.244.0.0/16"
    dnsServiceIPs: 
    - "10.96.0.10"
EOF
```

> Check networkProfile fields according to your installation
> To let Kamaji works in kind, you have indicate that the service must be [NodePort](https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport)

### Get Kubeconfig

Let's retrieve kubeconfig and store in `/tmp/kubeconfig`

```bash
$ kubectl get secrets tenant1-admin-kubeconfig -o json \
 | jq -r '.data["admin.conf"]' \
 | base64 -d > /tmp/kubeconfig
 ```

It can be export it, to facilitate the next tasks:

```bash
$ export KUBECONFIG=/tmp/kubeconfig
```

### Install CNI

We highly recommend to install [kindnet](https://github.com/aojea/kindnet) as CNI for your kamaji TCP.

```bash
$ kubectl create -f https://raw.githubusercontent.com/aojea/kindnet/master/install-kindnet.yaml
```

### Join worker nodes

```bash
$ make kamaji-kind-worker-join
```

> To add more worker nodes, run again the command above.

Check out the node:

```bash
$ kubectl get nodes
NAME           STATUS   ROLES    AGE   VERSION
d2d4b468c9de   Ready    <none>   44s   v1.23.4
```

> For more complex scenarios (exposing port, different version and so on), run `join-node.bash`

Tenant control plane provision has been finished in a minimal Kamaji setup based on KinD. Therefore, you could develop, test and make your own experiments with Kamaji.
