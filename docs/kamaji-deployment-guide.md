# Setup Kamaji
This guide will lead you through the process of creating a working Kamaji setup on a generic Kubernetes cluster. It requires:

- one bootstrap local workstation
- a Kubernetes cluster 1.22+, to run the Admin and Tenant Control Planes
- an arbitrary number of machines to host Tenants' workloads

> In this guide, we assume the machines are running `Ubuntu 20.04`.

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

## Access Admin cluster
In Kamaji, an Admin Cluster is a regular Kubernetes cluster which hosts zero to many Tenant Cluster Control Planes. The admin cluster acts as management cluster for all the Tenant clusters and implements Monitoring, Logging, and Governance of all the Kamaji setup, including all Tenant clusters. 

Throughout the following instructions, shell variables are used to indicate values that you should adjust to your environment:

```bash
source kamaji.env
```

Any regular and conformant Kubernetes v1.22+ cluster can be turned into a Kamaji setup. To work properly, the admin cluster should provide:

- CNI module installed, eg. [Calico](https://github.com/projectcalico/calico), [Cilium](https://github.com/cilium/cilium).
- CSI module installed with a Storage Class for the Tenants' `etcd`.
- Support for LoadBalancer Service Type, or alternatively, an Ingress Controller, eg. [ingress-nginx](https://github.com/kubernetes/ingress-nginx), [haproxy](https://github.com/haproxytech/kubernetes-ingress).
- Monitoring Stack, eg. [Prometheus](https://github.com/prometheus-community).

Make sure you have a `kubeconfig` file with admin permissions on the cluster you want to turn into Kamaji Admin Cluster.

## Install Kamaji
There are multiple ways to deploy Kamaji, including a [single YAML file](../config/install.yaml) and [Helm Chart](../helm/kamaji).

### Multi-tenant datastore
The Kamaji controller needs to access a multi-tenant datastore in order to save data of the tenants' clusters. Install a multi-tenant `etcd` in the admin cluster as three replicas StatefulSet with data persistence. The Helm [Chart](../helm/kamaji/) provides the installation of an internal `etcd`. However, an externally managed `etcd` is highly recommended. If you'd like to use an external one, you can specify the overrides by setting the value `etcd.deploy=false`.

Optionally, Kamaji offers the possibility of using a different storage system than `etcd` for the tenants' clusters, like MySQL compatible database, thanks to the [kine](https://github.com/k3s-io/kine) integration [here](../deploy/kine/mysql/README.md).

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

You just turned your Kubernetes cluster into a Kamaji cluster to run multiple Tenant Control Planes.

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
EOF

kubectl create namespace ${TENANT_NAMESPACE}
kubectl apply -f ${TENANT_NAMESPACE}-${TENANT_NAME}-tcp.yaml
```

After a few minutes, check the created resources in the tenants namespace and when ready it will look similar to the following:

```command
kubectl -n tenants get tcp,deploy,pods,svc
NAME                                             VERSION   STATUS   CONTROL-PLANE-ENDPOINT   KUBECONFIG                   AGE
tenantcontrolplane.kamaji.clastix.io/tenant-00   v1.23.1   Ready    192.168.32.240:6443      tenant-00-admin-kubeconfig   2m20s

NAME                        READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/tenant-00   3/3     3            3           118s

NAME                             READY   STATUS    RESTARTS   AGE
pod/tenant-00-58847c8cdd-7hc4n   4/4     Running   0          82s
pod/tenant-00-58847c8cdd-ft5xt   4/4     Running   0          82s
pod/tenant-00-58847c8cdd-shc7t   4/4     Running   0          82s

NAME                TYPE           CLUSTER-IP      EXTERNAL-IP      PORT(S)                         AGE
service/tenant-00   LoadBalancer   10.32.132.241   192.168.32.240   6443:32152/TCP,8132:32713/TCP   2m20s
```

The regular Tenant Control Plane containers: `kube-apiserver`, `kube-controller-manager`, `kube-scheduler` are running unchanged in the `tcp` pods instead of dedicated machines and they are exposed through a service on the port `6443` of worker nodes in the Admin cluster.

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

The `LoadBalancer` service type is used to expose the Tenant Control Plane. However, `NodePort` and `ClusterIP` with an Ingress Controller are still viable options, depending on the case. High Availability and rolling updates of the Tenant Control Plane are provided by the `tcp` Deployment and all the resources reconcilied by the Kamaji controller.

### Konnectivity
In addition to the standard control plane containers, Kamaji creates an instance of [konnectivity-server](https://kubernetes.io/docs/concepts/architecture/control-plane-node-communication/) running as sidecar container in the `tcp` pod and exposed on port `8132` of the `tcp` service.

This is required when the tenant worker nodes are not reachable from the `tcp` pods. The Konnectivity service consists of two parts: the Konnectivity server in the tenant control plane pod and the Konnectivity agents running on the tenant worker nodes. After worker nodes joined the tenant control plane, the Konnectivity agents initiate connections to the Konnectivity server and maintain the network connections. After enabling the Konnectivity service, all control plane to worker nodes traffic goes through these connections.

> In Kamaji, Konnectivity is enabled by default and can be disabled when not required.

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
  | base64 -d \
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

## Cleanup
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
