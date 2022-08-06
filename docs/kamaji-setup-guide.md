# Setup a Kamaji environment
This guide will lead you through the process of creating a working Kamaji setup.

Kamaji requires:

- a (optional) bootstrap machine;
- a regular Kubernetes cluster, to run the Admin and Tenant Control Planes
- an additional `etcd` cluster made of 3 replicas to host the datastore for the Tenants' clusters
- an arbitrary number of machines to host Tenants' workloads

> In this guide, we assume all machines are running `Ubuntu 20.04`.

  * [Prepare the bootstrap workspace](#prepare-the-bootstrap-workspace)
  * [Access Admin cluster](#access-admin-cluster)
  * [Setup multi-tenant etcd](#setup-multi-tenant-etcd)
  * [Install Kamaji controller](#install-kamaji-controller)

## Prepare the bootstrap workspace
This guide is supposed to be run from a remote or local bootstrap machine.
First, prepare the workspace directory:

```
git clone https://github.com/clastix/kamaji
cd kamaji/deploy
```

### Install required tools
On the bootstrap machine, install all the required tools to work with a Kamaji setup.

#### cfssl and cfssljson
The `cfssl` and `cfssljson` command line utilities will be used in addition to `kubeadm` to provision the PKI Infrastructure and generate TLS certificates.

```
wget -q --show-progress --https-only --timestamping \
https://storage.googleapis.com/kubernetes-the-hard-way/cfssl/1.4.1/linux/cfssl \
https://storage.googleapis.com/kubernetes-the-hard-way/cfssl/1.4.1/linux/cfssljson

chmod +x cfssl cfssljson
sudo mv cfssl cfssljson /usr/local/bin/
```

#### Kubernetes tools
Install `kubeadm` and `kubectl`

```bash
sudo apt update && sudo apt install -y apt-transport-https ca-certificates curl && \
sudo curl -fsSLo /usr/share/keyrings/kubernetes-archive-keyring.gpg https://packages.cloud.google.com/apt/doc/apt-key.gpg && \
echo "deb [signed-by=/usr/share/keyrings/kubernetes-archive-keyring.gpg] https://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee /etc/apt/sources.list.d/kubernetes.list && \
sudo apt update && sudo apt install -y kubeadm kubectl --allow-change-held-packages && \
sudo apt-mark hold kubeadm kubectl
```

#### etcdctl
For administration of the `etcd` cluster, download and install the `etcdctl` CLI utility on the bootstrap machine

```bash
ETCD_VER=v3.5.1
ETCD_URL=https://storage.googleapis.com/etcd
curl -L ${ETCD_URL}/${ETCD_VER}/etcd-${ETCD_VER}-linux-amd64.tar.gz -o etcd-${ETCD_VER}-linux-amd64.tar.gz
tar xzvf etcd-${ETCD_VER}-linux-amd64.tar.gz etcd-${ETCD_VER}-linux-amd64/etcdctl
sudo cp etcd-${ETCD_VER}-linux-amd64/etcdctl /usr/bin/etcdctl
rm -rf etcd-${ETCD_VER}-linux-amd64*
```

Verify `etcdctl` version is installed

```bash
etcdctl version
etcdctl version: 3.5.1
API version: 3.5
```


## Access Admin cluster
In Kamaji, an Admin Cluster is a regular Kubernetes cluster which hosts zero to many Tenant Cluster Control Planes. The admin cluster acts as management cluster for all the Tenant clusters and implements Monitoring, Logging, and Governance of all the Kamaji setup, including all Tenant clusters. 

Any regular and conformant Kubernetes v1.22+ cluster can be turned into a Kamaji setup. Currently we tested:

- [Kubernetes vanilla installed with `kubeadm`](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/).
- [AWS EKS managed service](./kamaji-on-aws.md)
- [Azure AKS managed service](./kamaji-on-azure.md)
- [KinD for local development](./getting-started-with-kamaji.md )

The admin cluster should provide:

- CNI module installed, eg. [Calico](https://github.com/projectcalico/calico), [Cilium](https://github.com/cilium/cilium).
- CSI module installed with a Storage Class for the Tenants' `etcd`.
- Support for LoadBalancer Service Type, or alternatively, an Ingress Controller, eg. [ingress-nginx](https://github.com/kubernetes/ingress-nginx), [haproxy](https://github.com/haproxytech/kubernetes-ingress).
- Monitoring Stack, eg. [Prometheus](https://github.com/prometheus-community).

Make sure you have a `kubeconfig` file with admin permissions on the cluster you want to turn into Kamaji Admin Cluster.

## Setup multi-tenant etcd

### Create certificates
From the bootstrap machine, use `kubeadm` init phase, to create the `etcd` CA certificates:

```bash
export ETCD_NAMESPACE=etcd-system
sudo kubeadm init phase certs etcd-ca
mkdir kamaji
sudo cp -r /etc/kubernetes/pki/etcd kamaji
sudo chown -R ${USER}. kamaji/etcd
```

Generate the `etcd` certificates for peers:

```
cat << EOF | tee kamaji/etcd/peer-csr.json
{
  "CN": "etcd",
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "hosts": [
    "127.0.0.1",
    "etcd-0",
    "etcd-0.etcd",
    "etcd-0.etcd.${ETCD_NAMESPACE}.svc",
    "etcd-0.etcd.${ETCD_NAMESPACE}.svc.cluster.local",
    "etcd-1",
    "etcd-1.etcd",
    "etcd-1.etcd.${ETCD_NAMESPACE}.svc",
    "etcd-1.etcd.${ETCD_NAMESPACE}.svc.cluster.local",
    "etcd-2",
    "etcd-2.etcd",
    "etcd-2.etcd.${ETCD_NAMESPACE}.svc",
    "etcd-2.etcd.${ETCD_NAMESPACE}.cluster.local"
  ]
}
EOF

cfssl gencert -ca=kamaji/etcd/ca.crt -ca-key=kamaji/etcd/ca.key \
  -config=cfssl-cert-config.json \
  -profile=peer-authentication kamaji/etcd/peer-csr.json | cfssljson -bare kamaji/etcd/peer

```

Generate the `etcd` certificates for server:

```
cat << EOF | tee kamaji/etcd/server-csr.json
{
  "CN": "etcd",
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "hosts": [
    "127.0.0.1",
    "etcd-server",
    "etcd-server.${ETCD_NAMESPACE}.svc",
    "etcd-server.${ETCD_NAMESPACE}.svc.cluster.local",
    "etcd-0.etcd.${ETCD_NAMESPACE}.svc.cluster.local",
    "etcd-1.etcd.${ETCD_NAMESPACE}.svc.cluster.local",
    "etcd-2.etcd.${ETCD_NAMESPACE}.svc.cluster.local"
  ]
}
EOF

cfssl gencert -ca=kamaji/etcd/ca.crt -ca-key=kamaji/etcd/ca.key \
  -config=cfssl-cert-config.json \
  -profile=peer-authentication kamaji/etcd/server-csr.json | cfssljson -bare kamaji/etcd/server
```

Generate certificates for the `root` user of the `etcd`

```
cat << EOF | tee kamaji/etcd/root-csr.json
{
  "CN": "root",
  "key": {
    "algo": "rsa",
    "size": 2048
  }
}
EOF

cfssl gencert -ca=kamaji/etcd/ca.crt -ca-key=kamaji/etcd/ca.key \
  -config=cfssl-cert-config.json \
  -profile=client-authentication kamaji/etcd/root-csr.json | cfssljson -bare kamaji/etcd/root	
```

Store the certificates of `etcd` into secrets:

```bash
kubectl create namespace ${ETCD_NAMESPACE}
kubectl -n ${ETCD_NAMESPACE} create secret generic etcd-certs \
  --from-file=kamaji/etcd/ca.crt \
  --from-file=kamaji/etcd/ca.key \
  --from-file=kamaji/etcd/peer-key.pem --from-file=kamaji/etcd/peer.pem \
  --from-file=kamaji/etcd/server-key.pem --from-file=kamaji/etcd/server.pem

kubectl -n ${ETCD_NAMESPACE} create secret tls root-client-certs \
  --key=kamaji/etcd/root-key.pem \
  --cert=kamaji/etcd/root.pem
```

### Create the etcd cluster
You can install tenants' `etcd` as StatefulSet in the Kamaji admin cluster. To achieve data persistency, make sure a Storage Class (default) is defined. Refer to the [documentation](https://etcd.io/docs/v3.5/op-guide/) for requirements and best practices to run `etcd` in production.

You should use topology spread constraints to control how `etcd` replicas are spread across the cluster among failure-domains such as regions, zones, nodes, and other user-defined topology domains. This helps to achieve high availability as well as efficient resource utilization. You can set cluster-level constraints as a default, or configure topology spread constraints by assigning the label `topology.kubernetes.io/zone` to the Kamaji admin cluster nodes hosting the tenants' `etcd`.

Install the tenants' `etcd` server:

```bash
kubectl -n ${ETCD_NAMESPACE} apply -f etcd/etcd-cluster.yaml
```

Install an `etcd` client to interact with the `etcd` server:

```bash
kubectl -n ${ETCD_NAMESPACE} apply -f etcd/etcd-client.yaml
```

Wait the `etcd` instances discover each other and the cluster is formed:

```bash
kubectl -n ${ETCD_NAMESPACE} exec etcd-root-client -- /bin/bash -c "etcdctl member list"
```

### Enable multi-tenancy
The `root` user has full access to `etcd`, must be created before activating authentication. The `root` user must have the `root` role and is allowed to change anything inside `etcd`.

```bash
kubectl -n ${ETCD_NAMESPACE} exec etcd-root-client -- etcdctl user add --no-password=true root
kubectl -n ${ETCD_NAMESPACE} exec etcd-root-client -- etcdctl role add root
kubectl -n ${ETCD_NAMESPACE} exec etcd-root-client -- etcdctl user grant-role root root
kubectl -n ${ETCD_NAMESPACE} exec etcd-root-client -- etcdctl auth enable
```

## Install Kamaji controller
Currently, the behaviour of the Kamaji controller for Tenant Control Plane is controlled by (in this order):

- CLI flags
- Environment variables
- Configuration file `kamaji.yaml` built into the image

By default Kamaji search for the configuration file and uses parameters found inside of it. In case some environment variable are passed, this will override configuration file parameters. In the end, if also a CLI flag is passed, this will override both env vars and config file as well.

There are multiple ways to deploy the Kamaji controller:

- Use the single YAML file installer
- Use Kustomize with Makefile
- Use the Kamaji Helm Chart

The Kamaji controller needs to access the multi-tenant `etcd` in order to provision the access for tenant `kube-apiserver`.

Create the secrets containing the `etcd` certificates

```bash
kubectl create namespace kamaji-system
kubectl -n kamaji-system create secret generic etcd-certs \
  --from-file=kamaji/etcd/ca.crt \
  --from-file=kamaji/etcd/ca.key

kubectl -n kamaji-system create secret tls root-client-certs \
  --cert=kamaji/etcd/root.pem \
  --key=kamaji/etcd/root-key.pem
```

### Install with a single manifest
Install with the single YAML file installer:

```bash
kubectl -n kamaji-system apply -f ../config/install.yaml
```

Make sure to patch the `etcd` endpoints of the Kamaji controller, according to your environment:

```bash
cat > patch-deploy.yaml <<EOF 
spec:
  template:
    spec:
      containers:
      - name: manager
        args:
        - --health-probe-bind-address=:8081
        - --metrics-bind-address=127.0.0.1:8080
        - --leader-elect
        - --etcd-endpoints=etcd-0.etcd.${ETCD_NAMESPACE}.svc.cluster.local:2379,etcd-1.etcd.${ETCD_NAMESPACE}.svc.cluster.local:2379,etcd-2.etcd.${ETCD_NAMESPACE}.svc.cluster.local:2379
EOF

kubectl -n kamaji-system patch \
  deployment kamaji-controller-manager \
  --patch-file patch-deploy.yaml
```

The Kamaji Tenant Control Plane controller is now running on the Admin Cluster:

```bash
kubectl -n kamaji-system get deploy
NAME                          READY   UP-TO-DATE   AVAILABLE   AGE
operator-controller-manager   1/1     1            1           14h
```

You turned the admin cluster into a Kamaji cluster able to run multiple Tenant Control Planes. Please, refer to the [Kamaji Tenant Deployment guide](./kamaji-tenant-deployment-guide.md) on how to.

