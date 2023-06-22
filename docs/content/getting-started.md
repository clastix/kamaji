# Getting started with Kamaji
This guide will lead you through the process of creating a working Kamaji setup on a generic infrastructure.

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

## Access Admin cluster
In Kamaji, an Admin Cluster is a regular Kubernetes cluster which hosts zero to many Tenant Cluster Control Planes. The admin cluster acts as management cluster for all the Tenant clusters and hosts monitoring, logging, and governance of Kamaji setup, including all Tenant clusters. 

Throughout the following instructions, shell variables are used to indicate values that you should adjust to your environment:

```bash
source kamaji.env
```

Any regular and conformant Kubernetes v1.22+ cluster can be turned into a Kamaji setup. To work properly, the admin cluster should provide:

- CNI module installed, eg. [Calico](https://github.com/projectcalico/calico), [Cilium](https://github.com/cilium/cilium).
- CSI module installed with a Storage Class for the Tenant datastores. Local Persistent Volumes are an option.
- Support for LoadBalancer service type, eg. [MetalLB](https://metallb.universe.tf/), or a Cloud based controller.
- Optionally, a Monitoring Stack installed, eg. [Prometheus](https://github.com/prometheus-community).

Make sure you have a `kubeconfig` file with admin permissions on the cluster you want to turn into Kamaji Admin Cluster and check you can access:

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

Installing Kamaji via Helm charts is the preferred way. The Kamaji controller needs to access a Datastore in order to save data of the tenants' clusters. The Kamaji Helm Chart provides the installation of a basic unamanaged `etcd` as datastore, out of box. 

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

A tenant control plane of example looks like:

```yaml
cat > ${TENANT_NAMESPACE}-${TENANT_NAME}-tcp.yaml <<EOF
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: ${TENANT_NAME}
  namespace: ${TENANT_NAMESPACE}
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
EOF

kubectl -n ${TENANT_NAMESPACE} apply -f ${TENANT_NAMESPACE}-${TENANT_NAME}-tcp.yaml
```

After a few seconds, check the created resources in the tenants namespace and when ready it will look similar to the following:

```command
kubectl -n ${TENANT_NAMESPACE} get tcp,deploy,pods,svc

NAME                           VERSION   STATUS   CONTROL-PLANE ENDPOINT   KUBECONFIG                   DATASTORE   AGE
tenantcontrolplane/tenant-00   v1.25.2   Ready    192.168.32.240:6443      tenant-00-admin-kubeconfig   default     2m20s

NAME                        READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/tenant-00   3/3     3            3           118s

NAME                             READY   STATUS    RESTARTS   AGE
pod/tenant-00-58847c8cdd-7hc4n   4/4     Running   0          82s
pod/tenant-00-58847c8cdd-ft5xt   4/4     Running   0          82s
pod/tenant-00-58847c8cdd-shc7t   4/4     Running   0          82s

NAME                TYPE           CLUSTER-IP      EXTERNAL-IP      PORT(S)                         AGE
service/tenant-00   LoadBalancer   10.32.132.241   192.168.32.240   6443:32152/TCP,8132:32713/TCP   2m20s
```

The regular Tenant Control Plane containers: `kube-apiserver`, `kube-controller-manager`, `kube-scheduler` are running unchanged in the `tcp` pods instead of dedicated machines and they are exposed through a service on the port `6443` of worker nodes in the admin cluster.

The `LoadBalancer` service type is used to expose the Tenant Control Plane on the assigned `loadBalancerIP` acting as `ControlPlaneEndpoint` for the worker nodes and other clients as, for example, `kubectl`. Service types `NodePort` and `ClusterIP` are still viable options to expose the Tenant Control Plane, depending on the case. High Availability and rolling updates of the Tenant Control Planes are provided by the `tcp` Deployment and all the resources reconcilied by the Kamaji controller.

### Working with Tenant Control Plane

Collect the external IP address of the `tcp` service:

```bash
TENANT_ADDR=$(kubectl -n ${TENANT_NAMESPACE} get svc ${TENANT_NAME} -o json | jq -r ."spec.loadBalancerIP")
```

and check it out:

```bash
curl -k https://${TENANT_ADDR}:${TENANT_PORT}/healthz
curl -k https://${TENANT_ADDR}:${TENANT_PORT}/version
```

The `kubeconfig` required to access the Tenant Control Plane is stored in a secret:

```bash
kubectl get secrets -n ${TENANT_NAMESPACE} ${TENANT_NAME}-admin-kubeconfig -o json \
  | jq -r '.data["admin.conf"]' \
  | base64 --decode \
  > ${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig
```

and let's check it out:

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig cluster-info

Kubernetes control plane is running at https://192.168.32.240:6443
CoreDNS is running at https://192.168.32.240:6443/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy
```

Check out how the Tenant control Plane advertises itself to workloads:

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get svc

NAMESPACE     NAME         TYPE        CLUSTER-IP   EXTERNAL-IP   PORT(S)   AGE
default       kubernetes   ClusterIP   10.32.0.1    <none>        443/TCP   6m
```

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get ep

NAME         ENDPOINTS             AGE
kubernetes   192.168.32.240:6443   18m
```

And make sure it is `${TENANT_ADDR}:${TENANT_PORT}`.

### Prepare worker nodes to join

Currently, Kamaji does not provide any helper for creation of tenant worker nodes.
You should get a set of machines from your infrastructure provider, turn them into worker nodes, and then join to the tenant control plane with the `kubeadm`.

Kamaji is sticking to the [Cluster Management API](https://github.com/kubernetes-sigs/cluster-api) project contracts by providing a `ControlPlane` provider.
Please, refer to the [official repository](https://github.com/clastix/cluster-api-control-plane-provider-kamaji) to learn more about it.

You can use the provided helper script `/deploy/nodes-prerequisites.sh`, in order to install the dependencies on all the worker nodes:

- Install `containerd` as container runtime
- Install `crictl`, the command line for working with `containerd`
- Install `kubectl`, `kubelet`, and `kubeadm` in the desired version

!!! warning ""
    The provided script is just a facility: it assumes all worker nodes are running `Ubuntu 20.04`. Make sure to adapt the script if you're using a different distribution.

Run the script:

```bash
HOSTS=(${WORKER0} ${WORKER1} ${WORKER2})
./nodes-prerequisites.sh ${TENANT_VERSION:1} ${HOSTS[@]}
```

### Join worker nodes
The current approach for joining nodes is to use `kubeadm` and therefore, we will create a bootstrap token to perform the action. In order to facilitate the step, we will store the entire command of joining in a variable:

```bash
JOIN_CMD=$(echo "sudo ")$(kubeadm --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig token create --print-join-command)
```

A bash loop will be used to join all the available nodes.

```bash
HOSTS=(${WORKER0} ${WORKER1} ${WORKER2})
for i in "${!HOSTS[@]}"; do
  HOST=${HOSTS[$i]}
  ssh ${USER}@${HOST} -t ${JOIN_CMD};
done
```

Checking the nodes:

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get nodes 

NAME                  STATUS     ROLES    AGE   VERSION
tenant-00-worker-00   NotReady   <none>   25s   v1.25.0
tenant-00-worker-01   NotReady   <none>   17s   v1.25.0
tenant-00-worker-02   NotReady   <none>   9s    v1.25.0
```

The cluster needs a [CNI](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/network-plugins/) plugin to get the nodes ready. In this guide, we are going to install [calico](https://projectcalico.docs.tigera.io/about/about-calico), but feel free to use one of your taste.

Download the latest stable Calico manifest:

```bash
curl https://raw.githubusercontent.com/projectcalico/calico/v3.24.1/manifests/calico.yaml -O
```

Before to apply the Calico manifest, you can customize it as necessary according to your preferences.

Apply to the tenant cluster:

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig apply -f calico.yaml
```

And after a while, nodes will be ready

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get nodes 
NAME                  STATUS   ROLES    AGE     VERSION
tenant-00-worker-00   Ready    <none>   2m48s   v1.25.0
tenant-00-worker-01   Ready    <none>   2m40s   v1.25.0
tenant-00-worker-02   Ready    <none>   2m32s   v1.25.0
```

## Cleanup
Remove the worker nodes joined the tenant control plane

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig delete nodes --all 
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

That's all folks!
