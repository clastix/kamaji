# Install a Kamaji environment
This guide will lead you through the process of creating a basic working Kamaji setup.

Kamaji requires:

- (optional) a bootstrap node;
- a multi-tenant `etcd` cluster made of 3 nodes hosting the datastore for the `Tenant`s' clusters
- a Kubernetes cluster, running the admin and Tenant Control Planes
- an arbitrary number of machines hosting `Tenant`s' workloads

> In this guide, we assume all machines are running `Ubuntu 20.04`.

  * [Prepare the bootstrap workspace](#prepare-the-bootstrap-workspace)
  * [Access Admin cluster](#access-admin-cluster)
  * [Setup external multi-tenant etcd](#setup-external-multi-tenant-etcd)
  * [Setup internal multi-tenant etcd](#setup-internal-multi-tenant-etcd)
  * [Install Kamaji controller](#install-kamaji-controller)
  * [Setup Tenant cluster](#setup-tenant-cluster)

## Prepare the bootstrap workspace
This guide is supposed to be run from a remote or local bootstrap machine.
First, prepare the workspace directory:

```
git clone https://github.com/clastix/kamaji
cd kamaji/deploy
```

Throughout the instructions, shell variables are used to indicate values that you should adjust to your own environment.

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
In Kamaji, an Admin Cluster is a regular Kubernetes cluster which hosts zero to many Tenant Cluster Control Planes running as pods. The admin cluster acts as management cluster for all the Tenant clusters and implements Monitoring, Logging, and Governance of all the Kamaji setup, including all Tenant clusters. 

Any regular and conformant Kubernetes v1.22+ cluster can be turned into a Kamaji setup. Currently we tested:

- [Kubernetes installed with `kubeadm`](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/).
- [Azure AKS managed service](./kamaji-on-azure.md).
- [KinD for local development](./getting-started-with-kamaji.md ).

The admin cluster should provide:

- CNI module installed, eg. Calico
- Support for LoadBalancer Service Type, eg. MetalLB or, alternatively, an Ingress Controller
- CSI module installed with StorageClass for multi-tenant `etcd`
- Monitoring Stack, eg. Prometheus and Grafana

Make sure you have a `kubeconfig` file with admin permissions on the cluster you want to turn into Kamaji Admin Cluster.

## Setup external multi-tenant etcd
In this section, we're going to setup a multi-tenant `etcd` cluster on dedicated nodes. Alternatively, if you want to use an internal `etcd` cluster as Kubernetes StatefulSet, jump [here](#setup-internal-multi-tenant-etcd).

### Ensure host access
From the bootstrap machine load the environment for external `etcd` setup:

```bash
source kamaji-external-etcd.env
```

The installer requires a user that has access to all hosts. In order to run the installer as a non-root user, first configure passwordless sudo rights each host:

Generate an SSH key on the host you run the installer on:

```bash
ssh-keygen -t rsa
```

> Do not use a password.

Distribute the key to the other cluster hosts.

Depending on your environment, use a bash loop:

```bash
HOSTS=(${ETCD0} ${ETCD1} ${ETCD2})
for i in "${!HOSTS[@]}"; do
  HOST=${HOSTS[$i]}
  ssh-copy-id -i ~/.ssh/id_rsa.pub $HOST;
done
```

> Alternatively, inject the generated public key into machines metadata.

Confirm that you can access each host from bootstrap machine:

```bash
HOSTS=(${ETCD0} ${ETCD1} ${ETCD2})
for i in "${!HOSTS[@]}"; do
  HOST=${HOSTS[$i]}
  ssh ${USER}@${HOST} -t 'hostname';
done
```

### Configure disk layout
As per `etcd` [requirements](https://etcd.io/docs/v3.5/op-guide/hardware/#disks), back `etcd`’s storage with a SSD. A SSD usually provides lower write latencies and with less variance than a spinning disk, thus improving the stability and reliability of `etcd`.

For each `etcd` machine, we assume an additional `sdb` disk of 10GB:

```
clastix@kamaji-etcd-00:~$ lsblk
NAME    MAJ:MIN RM  SIZE RO TYPE MOUNTPOINT
sda       8:0    0   16G  0 disk 
├─sda1    8:1    0 15.9G  0 part /
├─sda14   8:14   0    4M  0 part 
└─sda15   8:15   0  106M  0 part /boot/efi
sdb       8:16   0   10G  0 disk 
sr0      11:0    1    4M  0 rom  
```

Create partition, format, and mount the `etcd` disk, by running the script below from the bootstrap machine:

> If you already used the `etcd` disks, please make sure to wipe the partitions with `sudo wipefs --all --force /dev/sdb` before to attempt to recreate them.

```bash
for i in "${!ETCDHOSTS[@]}"; do
  HOST=${ETCDHOSTS[$i]}
  ssh ${USER}@${HOST} -t 'echo type=83 | sudo sfdisk -f -q /dev/sdb'
  ssh ${USER}@${HOST} -t 'sudo mkfs -F -q -t ext4 /dev/sdb1'
  ssh ${USER}@${HOST} -t 'sudo mkdir -p /var/lib/etcd'
  ssh ${USER}@${HOST} -t 'sudo e2label /dev/sdb1 ETCD'
  ssh ${USER}@${HOST} -t 'echo LABEL=ETCD /var/lib/etcd ext4 defaults 0 1 | sudo tee -a /etc/fstab'
  ssh ${USER}@${HOST} -t 'sudo mount -a'
  ssh ${USER}@${HOST} -t 'sudo lsblk -f'
done
```

### Install prerequisites
Use bash script `nodes-prerequisites.sh` to install all the dependencies on all the cluster nodes:

- Install `containerd` as container runtime
- Install `crictl`, the command line for working with `containerd`
- Install `kubectl`, `kubelet`, and `kubeadm` in the desired version, eg. `v1.24.0`

Run the installation script:

```bash
VERSION=v1.24.0
./nodes-prerequisites.sh ${VERSION:1} ${HOSTS[@]}
```

### Configure kubelet

On each `etcd` node, configure the `kubelet` service to start `etcd` static pods using `containerd` as container runtime, by running the script below from the bootstrap machine:

```bash
cat << EOF > 20-etcd-service-manager.conf
[Service]
ExecStart=
ExecStart=/usr/bin/kubelet --address=127.0.0.1 --pod-manifest-path=/etc/kubernetes/manifests --cgroup-driver=systemd --container-runtime=remote --container-runtime-endpoint=/run/containerd/containerd.sock
Restart=always
EOF
```

```
for i in "${!ETCDHOSTS[@]}"; do
  HOST=${ETCDHOSTS[$i]}
  scp 20-etcd-service-manager.conf ${USER}@${HOST}:
  ssh ${USER}@${HOST} -t 'sudo chown -R root:root 20-etcd-service-manager.conf && sudo mv 20-etcd-service-manager.conf /etc/systemd/system/kubelet.service.d/20-etcd-service-manager.conf'
  ssh ${USER}@${HOST} -t 'sudo systemctl daemon-reload'
  ssh ${USER}@${HOST} -t 'sudo systemctl start kubelet'
  ssh ${USER}@${HOST} -t 'sudo systemctl enable kubelet'
done

rm -f 20-etcd-service-manager.conf
```

### Create configuration
Create temp directories to store files that will end up on `etcd` hosts:

```bash
mkdir -p /tmp/${ETCD0}/ /tmp/${ETCD1}/ /tmp/${ETCD2}/
NAMES=("etcd00" "etcd01" "etcd02")

for i in "${!ETCDHOSTS[@]}"; do
HOST=${ETCDHOSTS[$i]}
NAME=${NAMES[$i]}

cat <<EOF | sudo tee /tmp/${HOST}/kubeadmcfg.yaml
apiVersion: "kubeadm.k8s.io/v1beta2"
kind: ClusterConfiguration
etcd:
  local:
    serverCertSANs:
    - "${HOST}"
    peerCertSANs:
    - "${HOST}"
    extraArgs:
      initial-cluster: ${NAMES[0]}=https://${ETCDHOSTS[0]}:2380,${NAMES[1]}=https://${ETCDHOSTS[1]}:2380,${NAMES[2]}=https://${ETCDHOSTS[2]}:2380
      initial-cluster-state: new
      name: ${NAME}
      listen-peer-urls: https://${HOST}:2380
      listen-client-urls: https://${HOST}:2379
      advertise-client-urls: https://${HOST}:2379
      initial-advertise-peer-urls: https://${HOST}:2380
      auto-compaction-mode: periodic
      auto-compaction-retention: 5m
      quota-backend-bytes: '8589934592'
EOF
done
```
> Note:
>
> ##### Etcd compaction
>
> By enabling `etcd` authentication, it prevents the tenant apiservers (clients of `etcd`) to issue compaction requests. We set `etcd` to automatically compact the keyspace with the `--auto-compaction-*` option with a period of hours or minutes. When `--auto-compaction-mode=periodic` and `--auto-compaction-retention=5m` and writes per minute are about 1000, `etcd` compacts revision 5000 for every 5 minute. 
> 
> ##### Etcd storage quota
>
> Currently, `etcd` is limited in storage size, defaulted to `2GB` and configurable with `--quota-backend-bytes` flag up to `8GB`. In Kamaji, we use a single `etcd` to store multiple tenant clusters, so we need to increase this size. Please, note `etcd` warns at startup if the configured value exceeds `8GB`.

### Generate certificates 
On the bootstrap machine, using `kubeadm` init phase, create and distribute `etcd` CA certificates:

```bash
sudo kubeadm init phase certs etcd-ca
mkdir kamaji
sudo cp -r /etc/kubernetes/pki/etcd kamaji
sudo chown -R ${USER}. kamaji/etcd
```

For each `etcd` host: 

```bash
for i in "${!ETCDHOSTS[@]}"; do
  HOST=${ETCDHOSTS[$i]}
  sudo kubeadm init phase certs etcd-server --config=/tmp/${HOST}/kubeadmcfg.yaml
  sudo kubeadm init phase certs etcd-peer --config=/tmp/${HOST}/kubeadmcfg.yaml
  sudo kubeadm init phase certs etcd-healthcheck-client --config=/tmp/${HOST}/kubeadmcfg.yaml
  sudo cp -R /etc/kubernetes/pki /tmp/${HOST}/
  sudo find /etc/kubernetes/pki -not -name ca.crt -not -name ca.key -type f -delete
done
```

### Startup the cluster
Upload certificates on each `etcd` node and restart the `kubelet`

```bash
for i in "${!ETCDHOSTS[@]}"; do
  HOST=${ETCDHOSTS[$i]}
  sudo chown -R ${USER}. /tmp/${HOST}
  scp -r /tmp/${HOST}/* ${USER}@${HOST}:
  ssh ${USER}@${HOST} -t 'sudo chown -R root:root pki'
  ssh ${USER}@${HOST} -t 'sudo mv pki /etc/kubernetes/'
  ssh ${USER}@${HOST} -t 'sudo kubeadm init phase etcd local --config=kubeadmcfg.yaml'
  ssh ${USER}@${HOST} -t 'sudo systemctl daemon-reload'
  ssh ${USER}@${HOST} -t 'sudo systemctl restart kubelet'
done
```

This will start the static `etcd` pod on each node and then the cluster gets formed.

Generate certificates for the `root` user

```bash
cat > root-csr.json <<EOF  
{
  "CN": "root",
  "key": {
    "algo": "rsa",
    "size": 2048
  }
}
EOF
```

```bash
cfssl gencert \
  -ca=kamaji/etcd/ca.crt \
  -ca-key=kamaji/etcd/ca.key \
  -config=cfssl-cert-config.json \
  -profile=client-authentication \
  root-csr.json | cfssljson -bare root
```

```bash
cp root.pem kamaji/etcd/root.crt
cp root-key.pem kamaji/etcd/root.key
rm root*
```

The result should be:

```bash
$ tree kamaji
kamaji
└── etcd
    ├── ca.crt
    ├── ca.key
    ├── root.crt
    └── root.key
```

Use the `root` user to check the just formed `etcd` cluster is in health state 

```bash
export ETCDCTL_CACERT=kamaji/etcd/ca.crt
export ETCDCTL_CERT=kamaji/etcd/root.crt
export ETCDCTL_KEY=kamaji/etcd/root.key
export ETCDCTL_ENDPOINTS=https://${ETCD0}:2379

etcdctl member list -w table
```

The result should be something like this:

```
+------------------+---------+--------+----------------------------+----------------------------+------------+
|        ID        | STATUS  |  NAME  |         PEER ADDRS         |        CLIENT ADDRS        | IS LEARNER |
+------------------+---------+--------+----------------------------+----------------------------+------------+
| 72657d6307364226 | started | etcd01 | https://192.168.32.11:2380 | https://192.168.32.11:2379 |      false |
| 91eb892c5ee87610 | started | etcd00 | https://192.168.32.10:2380 | https://192.168.32.10:2379 |      false |
| e9971c576949c34e | started | etcd02 | https://192.168.32.12:2380 | https://192.168.32.12:2379 |      false |
+------------------+---------+--------+----------------------------+----------------------------+------------+
```

### Enable multi-tenancy
The `root` user has full access to `etcd`, must be created before activating authentication. The `root` user must have the `root` role and is allowed to change anything inside `etcd`.

```bash
etcdctl user add --no-password=true root
etcdctl role add root
etcdctl user grant-role root root
etcdctl auth enable
```

### Cleanup
If you want to get rid of the etcd cluster, for each node, login and clean it:

```bash
HOSTS=(${ETCD0} ${ETCD1} ${ETCD2})
for i in "${!HOSTS[@]}"; do
  HOST=${HOSTS[$i]}
  ssh ${USER}@${HOST} -t 'sudo kubeadm reset -f';
  ssh ${USER}@${HOST} -t 'sudo systemctl reboot';
done
```

## Setup internal multi-tenant etcd
If you opted for an internal etcd cluster running in the Kamaji admin cluster, follow steps below.

From the bootstrap machine load the environment for internal `etcd` setup:

```bash
source kamaji-internal-etcd.env
```

### Generate certificates 
On the bootstrap machine, using `kubeadm` init phase, create the `etcd` CA certificates:

```bash
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

Install the `etcd` in the Kamaji admin cluster

```bash
kubectl create namespace ${ETCD_NAMESPACE}

kubectl -n ${ETCD_NAMESPACE} create secret generic etcd-certs \
  --from-file=kamaji/etcd/ca.crt \
  --from-file=kamaji/etcd/ca.key \
  --from-file=kamaji/etcd/peer-key.pem --from-file=kamaji/etcd/peer.pem \
  --from-file=kamaji/etcd/server-key.pem --from-file=kamaji/etcd/server.pem

kubectl -n ${ETCD_NAMESPACE} apply -f etcd/etcd-cluster.yaml
```

Install an `etcd` client to interact with the `etcd` server

```bash
kubectl -n ${ETCD_NAMESPACE} create secret tls root-certs \
  --key=kamaji/etcd/root-key.pem \
  --cert=kamaji/etcd/root.pem

kubectl -n ${ETCD_NAMESPACE} apply -f etcd/etcd-client.yaml
```

Wait the etcd instances discover each other and the cluster is formed:

```bash
kubectl -n ${ETCD_NAMESPACE} wait pod --for=condition=ready -l app=etcd --timeout=120s
echo -n "\nChecking endpoint's health..."

kubectl -n ${ETCD_NAMESPACE} exec etcd-root-client -- /bin/bash -c "etcdctl endpoint health 1>/dev/null 2>/dev/null; until [ \$$? -eq 0 ]; do sleep 10; printf "."; etcdctl endpoint health 1>/dev/null 2>/dev/null; done;"
echo -n "\netcd cluster's health:\n"

kubectl -n ${ETCD_NAMESPACE} exec etcd-root-client -- /bin/bash -c "etcdctl endpoint health"
echo -n "\nWaiting for all members..."

kubectl -n ${ETCD_NAMESPACE} exec etcd-root-client -- /bin/bash -c "until [ \$$(etcdctl member list 2>/dev/null | wc -l) -eq 3 ]; do sleep 10; printf '.'; done;"
	@echo -n "\netcd's members:\n"

kubectl -n ${ETCD_NAMESPACE} exec etcd-root-client -- /bin/bash -c "etcdctl member list -w table"
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
  --cert=kamaji/etcd/root.crt \
  --key=kamaji/etcd/root.key
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
        - --etcd-endpoints=${ETCD0}:2379,${ETCD1}:2379,${ETCD2}:2379
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

## Setup Tenant Cluster
Now you are getting an Admin Cluster available to run multiple Tenant Control Planes, deployed by the Kamaji controller. Please, refer to the Kamaji Tenant Deployment [guide](./kamaji-tenant-deployment-guide.md).

