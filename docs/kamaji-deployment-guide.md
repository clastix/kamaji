# Setup Kamaji
This guide will lead you through the process of creating a working Kamaji setup on a generic Kubernetes cluster It requires:

- one bootstrap local workstation
- a Kubernetes cluster, to run the Admin and Tenant Control Planes
- an additional `etcd` cluster made of 3 replicas to host the datastore for the Tenants' clusters
- an arbitrary number of machines to host Tenants' workloads

> In this guide, we assume all machines are running `Ubuntu 20.04`.

  * [Prepare the bootstrap workspace](#prepare-the-bootstrap-workspace)
  * [Access Admin cluster](#access-admin-cluster)
  * [Setup multi-tenant etcd](#setup-multi-tenant-etcd)
  * [Install Kamaji controller](#install-kamaji-controller)
  * [Create Tenant Cluster](#create-tenant-cluster)
  * [Cleanup](#cleanup)

## Prepare the bootstrap workspace
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

Any regular and conformant Kubernetes v1.22+ cluster can be turned into a Kamaji setup. To work properly, the admin cluster should provide:

- CNI module installed, eg. [Calico](https://github.com/projectcalico/calico), [Cilium](https://github.com/cilium/cilium).
- CSI module installed with a Storage Class for the Tenants' `etcd`.
- Support for LoadBalancer Service Type, or alternatively, an Ingress Controller, eg. [ingress-nginx](https://github.com/kubernetes/ingress-nginx), [haproxy](https://github.com/haproxytech/kubernetes-ingress).
- Monitoring Stack, eg. [Prometheus](https://github.com/prometheus-community).

Make sure you have a `kubeconfig` file with admin permissions on the cluster you want to turn into Kamaji Admin Cluster.

Throughout the following instructions, shell variables are used to indicate values that you should adjust to your environment:

```bash
source kamaji.env
```

## Setup multi-tenant etcd

### Create certificates
From the bootstrap machine, use `kubeadm` init phase, to create the `etcd` CA certificates:

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

You turned your cluster into a Kamaji cluster to run multiple Tenant Control Planes.




## Create Tenant Cluster

### Create a tenant control plane

Create a tenant control plane of example

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
        controllerManager:
          requests:
            cpu: 250m
            memory: 256Mi
        scheduler:
          requests:
            cpu: 250m
            memory: 256Mi
    service:
      additionalMetadata:
        labels:
          tenant.clastix.io: ${TENANT_NAME}
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
EOF
```

Kamaji implements [konnectivity](https://kubernetes.io/docs/concepts/architecture/control-plane-node-communication/) as sidecar container of the tenant control plane pod and it is exposed using the same service on port `8132`. It's required when workers are directly not reachable from the tenant control plane, and it's enabled by default.

```bash
kubectl create namespace ${TENANT_NAMESPACE}
kubectl apply -f ${TENANT_NAMESPACE}-${TENANT_NAME}-tcp.yaml
```

A tenant control plane control plane is now running and it is exposed through a service like this:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: tenant-00
spec:
  clusterIP: 10.32.233.177
  loadBalancerIP: 192.168.32.240
  ports:
  - name: kube-apiserver
    nodePort: 31073
    port: 6443
    protocol: TCP
    targetPort: 6443
  - name: konnectivity-server
    nodePort: 32125
    port: 8132
    protocol: TCP
    targetPort: 8132
  selector:
    kamaji.clastix.io/soot: tenant-00
  type: LoadBalancer
```

### Working with Tenant Control Plane

Collect the IP address of the loadbalancer service where the Tenant control Plane is exposed:

```bash
TENANT_ADDR=$(kubectl -n ${TENANT_NAMESPACE} get svc ${TENANT_NAME} -o json | jq -r ."status.loadBalancer.ingress[].ip")
```

and check it out:

```bash
curl -k https://${TENANT_ADDR}:${TENANT_PORT}/healthz
curl -k https://${TENANT_ADDR}:${TENANT_PORT}/version
```

Let's retrieve the `kubeconfig` in order to work with the tenant control plane.

```bash
kubectl get secrets -n ${TENANT_NAMESPACE} ${TENANT_NAME}-admin-kubeconfig -o json \
  | jq -r '.data["admin.conf"]' \
  | base64 -d \
  > ${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig
```

and let's check it out:

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get svc

NAMESPACE     NAME         TYPE        CLUSTER-IP   EXTERNAL-IP   PORT(S)   AGE
default       kubernetes   ClusterIP   10.32.0.1    <none>        443/TCP   6m
```

Check out how the Tenant control Plane advertises itself to workloads:

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get ep

NAME         ENDPOINTS             AGE
kubernetes   192.168.32.240:6443   18m
```

### Preparing Worker Nodes to join

Currently Kamaji does not provide any helper for creation of tenant worker nodes. You should get a set of machines from your infrastructure provider, turn them into worker nodes, and then join to the tenant control plane with the `kubeadm`. In the future, we'll provide integration with Cluster APIs and other IaC tools.

Use bash script `nodes-prerequisites.sh` to install the dependencies on all the worker nodes:

- Install `containerd` as container runtime
- Install `crictl`, the command line for working with `containerd`
- Install `kubectl`, `kubelet`, and `kubeadm` in the desired version

> Warning: we assume worker nodes are machines running `Ubuntu 20.04`

Run the installation script:

```bash
HOSTS=(${WORKER0} ${WORKER1} ${WORKER2})
./nodes-prerequisites.sh ${TENANT_VERSION:1} ${HOSTS[@]}
```

### Join Command

The current approach for joining nodes is to use the kubeadm one therefore, we will create a bootstrap token to perform the action. In order to facilitate the step, we will store the entire command of joining in a variable.

```bash
JOIN_CMD=$(echo "sudo ")$(kubeadm --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig token create --print-join-command)
```

### Adding Worker Nodes

A bash loop will be used to join all the available nodes.

```bash
for i in "${!HOSTS[@]}"; do
  HOST=${HOSTS[$i]}
  ssh ${USER}@${HOST} -t ${JOIN_CMD};
done
```

Checking the nodes:

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get nodes 

NAME                  STATUS     ROLES    AGE   VERSION
tenant-00-worker-00   NotReady   <none>   25s   v1.23.5
tenant-00-worker-01   NotReady   <none>   17s   v1.23.5
tenant-00-worker-02   NotReady   <none>   9s    v1.23.5
```

The cluster needs a [CNI](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/network-plugins/) plugin to get the nodes ready. In our case, we are going to install [calico](https://projectcalico.docs.tigera.io/about/about-calico).

```bash
kubectl apply -f calico-cni/calico-crd.yaml --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig
kubectl apply -f calico-cni/calico.yaml --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig
```

And after a while, `kube-system` pods will be running.

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get pods -n kube-system 

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
NAME                  STATUS   ROLES    AGE     VERSION
tenant-00-worker-00   Ready    <none>   2m48s   v1.23.5
tenant-00-worker-01   Ready    <none>   2m40s   v1.23.5
tenant-00-worker-02   Ready    <none>   2m32s   v1.23.5
```

## Smoke test

The tenant cluster is now ready to accept workloads.

Export its `kubeconfig` file

```bash
export KUBECONFIG=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig
```

#### Deployment
Deploy a `nginx` application on the tenant cluster

```bash
kubectl create deployment nginx --image=nginx
```

and check the `nginx` pod gets scheduled

```bash
kubectl get pods -o wide

NAME                     READY   STATUS    RESTARTS   AGE   IP             NODE     
nginx-6799fc88d8-4sgcb   1/1     Running   0          33s   172.12.121.1   worker02
```

#### Port Forwarding
Verify the ability to access applications remotely using port forwarding.

Retrieve the full name of the `nginx` pod:

```bash
POD_NAME=$(kubectl get pods -l app=nginx -o jsonpath="{.items[0].metadata.name}")
```

Forward port 8080 on your local machine to port 80 of the `nginx` pod:

```bash
kubectl port-forward $POD_NAME 8080:80

Forwarding from 127.0.0.1:8080 -> 80
Forwarding from [::1]:8080 -> 80
```

In a new terminal make an HTTP request using the forwarding address:

```bash
curl --head http://127.0.0.1:8080

HTTP/1.1 200 OK
Server: nginx/1.21.0
Date: Sat, 19 Jun 2021 08:19:01 GMT
Content-Type: text/html
Content-Length: 612
Last-Modified: Tue, 25 May 2021 12:28:56 GMT
Connection: keep-alive
ETag: "60aced88-264"
Accept-Ranges: bytes
```

Switch back to the previous terminal and stop the port forwarding to the `nginx` pod.

#### Logs
Verify the ability to retrieve container logs.

Print the `nginx` pod logs:

```bash
kubectl logs $POD_NAME
...
127.0.0.1 - - [19/Jun/2021:08:19:01 +0000] "HEAD / HTTP/1.1" 200 0 "-" "curl/7.68.0" "-"
```

#### Kubelet tunnel
Verify the ability to execute commands in a container.

Print the `nginx` version by executing the `nginx -v` command in the `nginx` container:

```bash
kubectl exec -ti $POD_NAME -- nginx -v
nginx version: nginx/1.21.0
```

#### Services
Verify the ability to expose applications using a service.

Expose the `nginx` deployment using a `NodePort` service:

```bash
kubectl expose deployment nginx --port 80 --type NodePort
```

Retrieve the node port assigned to the `nginx` service:

```bash
NODE_PORT=$(kubectl get svc nginx \
  --output=jsonpath='{range .spec.ports[0]}{.nodePort}')
```

Retrieve the IP address of a worker instance and make an HTTP request:

```bash
curl -I http://${WORKER0}:${NODE_PORT}

HTTP/1.1 200 OK
Server: nginx/1.21.0
Date: Sat, 19 Jun 2021 09:29:01 GMT
Content-Type: text/html
Content-Length: 612
Last-Modified: Tue, 25 May 2021 12:28:56 GMT
Connection: keep-alive
ETag: "60aced88-264"
Accept-Ranges: bytes
```

## Cleanup Tenant cluster
Remove the worker nodes joined the tenant control plane

```bash
kubectl delete nodes --all --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig
```

For each worker node, login and clean it

```bash
HOSTS=(${WORKER0} ${WORKER1} ${WORKER2})
for i in "${!HOSTS[@]}"; do
  HOST=${HOSTS[$i]}
  ssh ${USER}@${HOST} -t 'sudo kubeadm reset -f';
  ssh ${USER}@${HOST} -t 'sudo rm -rf /etc/cni/net.d';
  ssh ${USER}@${HOST} -t 'sudo systemctl reboot';
done
```

Delete the tenant control plane from kamaji

```bash
kubectl delete -f ${TENANT_NAMESPACE}-${TENANT_NAME}-tcp.yaml
```
