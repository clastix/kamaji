# Kamaji Tenant Deployment Guide

This guide defines the necessary actions to generate a kubernetes tenant cluster, which can be considered made of a virtual kubernetes control plane, deployed by Kamaji, and joining worker nodes pool to start workloads.

## Requirements

* [Kubernetes](https://kubernetes.io) Admin Cluster having [Kamaji](./getting-started-with-kamaji.md) installed.
* [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
* [jq](https://stedolan.github.io/jq/)

## Tenant Control Plane

Kamaji offers a [CRD](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) to provide a declarative approach of managing tenant control planes. This *CRD* is called `TenantControlPlane`, or `tcp` in short.

Use the command `kubectl explain tcp.spec` to understand the fields and their usage.

### Variable Definitions
Throughout the instructions, shell variables are used to indicate values that you should adjust to your own environment:

```bash
source kamaji-tenant.env
```

### Creation

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
    address: ${TENANT_ADDR}
    port: ${TENANT_PORT}
    domain: ${TENANT_DOMAIN}
    serviceCidr: ${TENANT_SVC_CIDR}
    podCidr: ${TENANT_POD_CIDR}
    dnsServiceIPs:
    - ${TENANT_DNS_SERVICE}
  addons:
    coreDNS:
      enabled: true
    kubeProxy:
      enabled: true
EOF
```

```bash
kubectl create namespace ${TENANT_NAMESPACE}
kubectl apply -f ${TENANT_NAMESPACE}-${TENANT_NAME}-tcp.yaml
```

A tenant control plane control plane is now running as deployment and it is exposed through a service.

Check if control plane of the tenant is reachable and in healty state

```bash
curl -k https://${TENANT_ADDR}:${TENANT_PORT}/healthz
curl -k https://${TENANT_ADDR}:${TENANT_PORT}/version
```

The tenant control plane components, i.e. `kube-apiserver`, `kube-scheduler`, and `kube-controller-manager` are running as containers in the same pods. The `kube-scheduler`, and `kube-controller-manager` connect the `kube-apiserver` throught localhost: `https://127.0.0.1.${TENANT_PORT}`

Let's retrieve the `kubeconfig` files in order to check:

```bash
kubectl get secrets -n ${TENANT_NAMESPACE} ${TENANT_NAME}-scheduler-kubeconfig -o json \
  | jq -r '.data["scheduler.conf"]' \
  | base64 -d \
  > ${TENANT_NAMESPACE}-${TENANT_NAME}-scheduler.kubeconfig
```

```bash
kubectl get secrets -n ${TENANT_NAMESPACE} ${TENANT_NAME}-controller-manager-kubeconfig -o json \
  | jq -r '.data["controller-manager.conf"]' \
  | base64 -d \
  > ${TENANT_NAMESPACE}-${TENANT_NAME}-controller-manager.kubeconfig
```

## Working with Tenant Control Plane

A new Tenant cluster will be available at this moment but, it will not be useful without having worker nodes joined to it.

### Getting Tenant Control Plane Kubeconfig

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
kubernetes   192.168.32.150:6443   18m
```

Make sure it's `${TENANT_ADDR}:${TENANT_PORT}`.

### Preparing Worker Nodes to join

Currently Kamaji does not provide any helper for creation of tenant worker nodes. You should get a set of machines from your infrastructure provider, turn them into worker nodes, and then join to the tenant control plane with the `kubeadm`. In the future, we'll provide integration with Cluster APIs and other IaC tools.

Use bash script `nodes-prerequisites.sh` to install the dependencies on all the worker nodes:

- Install `containerd` as container runtime
- Install `crictl`, the command line for working with `containerd`
- Install `kubectl`, `kubelet`, and `kubeadm` in the desired version

> Warning: we assume worker nodes are machines running `Ubuntu 20.04`

Run the installation script:

```bash
HOSTS=(${WORKER0} ${WORKER1} ${WORKER2} ${WORKER3})
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
HOSTS=(${WORKER0} ${WORKER1} ${WORKER2} ${WORKER3})
for i in "${!HOSTS[@]}"; do
  HOST=${HOSTS[$i]}
  ssh ${USER}@${HOST} -t ${JOIN_CMD};
done
```

Checking the nodes:

```bash
kubectl --kubeconfig=${TENANT_NAMESPACE}-${TENANT_NAME}.kubeconfig get nodes 

NAME                      STATUS      ROLES    AGE    VERSION
kamaji-tenant-worker-00   NotReady    <none>   1m     v1.23.4
kamaji-tenant-worker-01   NotReady    <none>   1m     v1.23.4
kamaji-tenant-worker-02   NotReady    <none>   1m     v1.23.4
kamaji-tenant-worker-03   NotReady    <none>   1m     v1.23.4
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

NAME                      STATUS      ROLES    AGE    VERSION
kamaji-tenant-worker-01   Ready       <none>   10m    v1.23.4
kamaji-tenant-worker-02   Ready       <none>   10m    v1.23.4
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
HOSTS=(${WORKER0} ${WORKER1} ${WORKER2} ${WORKER3})
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
