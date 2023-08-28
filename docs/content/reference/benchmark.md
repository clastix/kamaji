# Benchmark

Kamaji has been designed to operate a large scale of Kubernetes Tenant Control Plane resources.

In the Operator jargon, a manager is created to start several controllers, each one with their own responsibility.
When a manager is started, all the underlying controllers are started, along with other "runnable" resources, like the webhook server.

Kamaji operates several reconciliation operations, both in the admin and Tenant Clusters.
With that said, a main manager is responsible to reconcile the admin resources (Deployment, Secret, ConfigMap, etc.), for each Tenant Control Plane a new manager will be spin-up as a main manager controller.
These Tenant Control Plane managers, named in the code base as soot managers, in turn, start and run controllers to ensure the desired state of the underlying add-ons, and required resources such as kubeadm ones.

With that said, monitoring the Kamaji stack is essential to understand any anomaly in memory consumption, or CPU usage.
The provided Helm Chart is offering a [`ServiceMonitor`](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/user-guides/getting-started.md) that can be used to extract all the required metrics of the Kamaji operator.

# Running 100 Tenant Control Planes using a single DataStore

- _Cloud platform:_ AWS
- _Amount of reconciled TCPs:_ 100
- _Number of DataStore resources:_ 1
- _DataStore driver_: `etcd`
- _Reconciliation time requested:_ ~7m30s

The following benchmark wants to shed a light on the Kamaji resource consumption such as CPU and memory to orchestrate 100 TCPs using a single shared datastore.
Your mileage may vary and just want to share with the community how it has been elaborated.

## Infrastructure

The benchmark has been issued on a Kubernetes cluster backed by Elastic Kubernetes Service used as Management Cluster.

Two node pools have been created to avoid the noisy neighbour effect, and to increase the performances:

- `infra`: hosting Kamaji, the monitoring stack, and the `DataStore` resources, made of 2 `t3.medium` instances.
- `workload`: hosting the deployed Tenant Control Plane resources, made of 25 `r3.xlarge` instances.

### Monitoring stack

The following monitoring stack was required to perform the benchmark.

#### Deploy the Prometheus operator

```
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
helm upgrade --install kube-prometheus bitnami/kube-prometheus --namespace monitoring-system --create-namespace \
    --set operator.nodeSelector."eks\.amazonaws\.com/nodegroup"=infra \
    --set "operator.tolerations[0].key=pool" \
    --set "operator.tolerations[0].operator=Equal" \
    --set "operator.tolerations[0].value=infra" \
    --set "operator.tolerations[0].effect=NoExecute" \
    --set "operator.tolerations[1].key=pool" \
    --set "operator.tolerations[1].operator=Equal" \
    --set "operator.tolerations[1].value=infra" \
    --set "operator.tolerations[1].effect=NoSchedule" \
    --set prometheus.nodeSelector."eks\.amazonaws\.com/nodegroup"=infra \
    --set "prometheus.tolerations[0].key=pool" \
    --set "prometheus.tolerations[0].operator=Equal" \
    --set "prometheus.tolerations[0].value=infra" \
    --set "prometheus.tolerations[0].effect=NoExecute" \
    --set "prometheus.tolerations[1].key=pool" \
    --set "prometheus.tolerations[1].operator=Equal" \
    --set "prometheus.tolerations[1].value=infra" \
    --set "prometheus.tolerations[1].effect=NoSchedule" \
    --set alertmanager.nodeSelector."eks\.amazonaws\.com/nodegroup"=infra \
    --set "alertmanager.tolerations[0].key=pool" \
    --set "alertmanager.tolerations[0].operator=Equal" \
    --set "alertmanager.tolerations[0].value=infra" \
    --set "alertmanager.tolerations[0].effect=NoExecute" \
    --set "alertmanager.tolerations[1].key=pool" \
    --set "alertmanager.tolerations[1].operator=Equal" \
    --set "alertmanager.tolerations[1].value=infra" \
    --set "alertmanager.tolerations[1].effect=NoSchedule" \
    --set kube-state-metrics.nodeSelector."eks\.amazonaws\.com/nodegroup"=infra \
    --set "kube-state-metrics.tolerations[0].key=pool" \
    --set "kube-state-metrics.tolerations[0].operator=Equal" \
    --set "kube-state-metrics.tolerations[0].value=infra" \
    --set "kube-state-metrics.tolerations[0].effect=NoExecute" \
    --set "kube-state-metrics.tolerations[1].key=pool" \
    --set "kube-state-metrics.tolerations[1].operator=Equal" \
    --set "kube-state-metrics.tolerations[1].value=infra" \
    --set "kube-state-metrics.tolerations[1].effect=NoSchedule" \
    --set blackboxExporter.nodeSelector."eks\.amazonaws\.com/nodegroup"=infra \
    --set "blackboxExporter.tolerations[0].key=pool" \
    --set "blackboxExporter.tolerations[0].operator=Equal" \
    --set "blackboxExporter.tolerations[0].value=infra" \
    --set "blackboxExporter.tolerations[0].effect=NoExecute" \
    --set "blackboxExporter.tolerations[1].key=pool" \
    --set "blackboxExporter.tolerations[1].operator=Equal" \
    --set "blackboxExporter.tolerations[1].value=infra" \
    --set "blackboxExporter.tolerations[1].effect=NoSchedule"
```

### Deploy a Grafana dashboard

```
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
helm upgrade --install grafana bitnami/grafana --namespace monitoring-system --create-namespace \
    --set grafana.nodeSelector."eks\.amazonaws\.com/nodegroup"=infra \
    --set "grafana.tolerations[0].key=pool" \
    --set "grafana.tolerations[0].operator=Equal" \
    --set "grafana.tolerations[0].value=infra" \
    --set "grafana.tolerations[0].effect=NoSchedule" \
    --set "grafana.tolerations[1].key=pool" \
    --set "grafana.tolerations[1].operator=Equal" \
    --set "grafana.tolerations[1].value=infra" \
    --set "grafana.tolerations[1].effect=NoExecute"

```

> The dashboard can be used to have a visual representation of the global cluster resources usage, and getting information about the single Pods resources consumption.
>
> Besides that, Grafana has been used to track the etcd cluster status and performances, although it's not the subject of the benchmark.

### Install cert-manager

```
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
helm upgrade --install cert-manager bitnami/cert-manager --namespace certmanager-system --create-namespace \
    --set cainjector.nodeSelector."eks\.amazonaws\.com/nodegroup"=infra \
    --set "cainjector.tolerations[0].key=pool" \
    --set "cainjector.tolerations[0].operator=Equal" \
    --set "cainjector.tolerations[0].value=infra" \
    --set "cainjector.tolerations[0].effect=NoSchedule" \
    --set "cainjector.tolerations[1].key=pool" \
    --set "cainjector.tolerations[1].operator=Equal" \
    --set "cainjector.tolerations[1].value=infra" \
    --set "cainjector.tolerations[1].effect=NoExecute" \
    --set controller.nodeSelector."eks\.amazonaws\.com/nodegroup"=infra \
    --set "controller.tolerations[0].key=pool" \
    --set "controller.tolerations[0].operator=Equal" \
    --set "controller.tolerations[0].value=infra" \
    --set "controller.tolerations[0].effect=NoExecute" \
    --set "controller.tolerations[1].key=pool" \
    --set "controller.tolerations[1].operator=Equal" \
    --set "controller.tolerations[1].value=infra" \
    --set "controller.tolerations[1].effect=NoSchedule" \
    --set webhook.nodeSelector."eks\.amazonaws\.com/nodegroup"=infra \
    --set "webhook.tolerations[0].key=pool" \
    --set "webhook.tolerations[0].operator=Equal" \
    --set "webhook.tolerations[0].value=infra" \
    --set "webhook.tolerations[0].effect=NoSchedule" \
    --set "webhook.tolerations[1].key=pool" \
    --set "webhook.tolerations[1].operator=Equal" \
    --set "webhook.tolerations[1].value=infra" \
    --set "webhook.tolerations[1].effect=NoExecute" \
    --set "installCRDs=true"
```

### Install Kamaji

```
helm upgrade --install kamaji clastix/kamaji --namespace kamaji-system --create-namespace \
    --set nodeSelector."eks\.amazonaws\.com/nodegroup"=infra \
    --set "tolerations[0].key=pool" \
    --set "tolerations[0].operator=Equal" \
    --set "tolerations[0].value=infra" \
    --set "tolerations[0].effect=NoExecute" \
    --set "tolerations[1].key=pool" \
    --set "tolerations[1].operator=Equal" \
    --set "tolerations[1].value=infra" \
    --set "tolerations[1].effect=NoSchedule" \
    --set "resources=null"
```

> For the benchmark, Kamaji is running without any resource constraint to benefit of all the available resources.

### Install etcd as a DataStore

```
helm upgrade --install etcd-01 clastix/kamaji-etcd --namespace kamaji-etcd --create-namespace --set "serviceMonitor.enabled=true" --set "datastore.enabled=true"
```

## Creating 100 Tenant Control Planes

Once all the required components have been deployed, a simple bash for loop can be used to deploy the TCP resources.

```
kubectl create ns benchmark01

for I in {001..100}; do
    DS=etcd-01 NS=benchmark01 I=$I envsubst < kamaji_v1alpha1_tenantcontrolplane.yaml | kubectl apply -f -
done
```

Content of the benchmark file:

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: benchmark$I
  namespace: $NS
  labels:
    tenant.clastix.io: benchmark$I
spec:
  dataStore: $DS
  controlPlane:
    deployment:
      replicas: 2
      resources:
        apiServer:
          requests:
            memory: 320Mi
            cpu: 100m
    service:
      serviceType: ClusterIP
  kubernetes:
    version: "v1.25.4"
    kubelet:
      cgroupfs: cgroupfs
    admissionControllers:
      - ResourceQuota
      - LimitRanger
  networkProfile:
    port: 6443
  addons:
    coreDNS: {}
    kubeProxy: {}
```

> For the benchmark we're not creating a LoadBalancer service, or an Ingress.
> The goal of the benchmark is to monitor resource consumption, and the average time required to create the requested resources.

## Conclusion

Our latest benchmark showed the ability to fully reconcile 100 Tenant Control Plane resources in ~7m30s.
All the Tenant Control Planes were in `Ready` state and able to handle any request.

The CPU consumption of Kamaji was fluctuating between 100 and 1200 mCPU during the peaks due to certificate generations.

The memory consumption of Kamaji hit ~600 MiB, although this data is not entirely representative since we didn't put any memory limit.

In conclusion, Kamaji was able to reconcile a Tenant Control Plane every 4.5 seconds, requiring 12 mCPU, and 6 MiB of memory.

The following values may vary according to the nodes, resource limits, and other constraints.
If you're encountering different results, please, engage with the community to share them. 

# Running a thousand of Tenant Control Planes using multiple DataStores

The next benchmark must address the use case where a Kamaji Management Cluster manages up to a thousand Tenant Control Plane instances.
