# Konnectivity

In traditional Kubernetes deployments, the control plane components need to communicate directly with worker nodes for various operations like:
executing commands in pods, retrieving logs, or managing port forwards.

However, in many real-world environments, especially those spanning multiple networks or cloud providers,
direct communication isn't always possible or desirable. This is where Konnectivity comes in.

## Understanding Konnectivity in Kamaji

Kamaji integrates [Konnectivity](https://kubernetes.io/docs/concepts/architecture/control-plane-node-communication/) as a core component of its architecture.
Each Tenant Control Plane pod includes a `konnectivity-server` running as a sidecar container,
which establishes and maintains secure tunnels with agents running on the worker nodes.

This design ensures reliable communication even in complex network environments.

The Konnectivity service consists of two main components:

1. **Konnectivity Server:**  
   Runs alongside the control plane components in each Tenant Control Plane pod and is exposed on port 8132. 
   It manages connections from worker nodes and routes traffic appropriately.

2. **Konnectivity Agent:**  
   Runs on worker nodes as _DaemonSet_ or _Deployment_ and initiates outbound connections to its control plane's Konnectivity server. 
   These connections are maintained to create a reliable tunnel for all control plane to worker node communications.

## How It Works

When a worker node joins a Tenant Cluster, the Konnectivity agents automatically establish connections to their designated Konnectivity server.
These connections are maintained continuously, ensuring reliable communication paths between the control plane and worker nodes.

All traffic from the control plane to worker nodes flows through these established tunnels, enabling operations such as:

- Executing commands in pods
- Retrieving container logs
- Managing port forwards
- Collecting metrics and health information
- Running exec sessions for debugging

## Configuration and Management

Konnectivity is enabled by default in Kamaji, as it's considered a best practice for modern Kubernetes deployments.
However, it can be disabled if your environment has different requirements, or if you need to use alternative networking solutions.

The service is automatically configured when worker nodes join a cluster, without requiring any operational overhead.
The connection details are managed as part of the standard node bootstrap process,
making it transparent to cluster operators and users.

## Agent delivery mode

You can customise the Konnectivity Agent delivery mode via the Tenant Control Plane definition
using the field `tenantcontrolplane.spec.addons.konnectivity.agent.mode`.

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: konnectivity-example
spec:
  controlPlane:
    deployment:
      replicas: 2
    service:
      serviceType: LoadBalancer
  kubernetes:
    version: "v1.33.0"
  networkProfile:
    port: 6443
  addons:
    konnectivity:
      server:
        port: 8132
      agent:
        ## DaemonSet, Deployment
        mode: DaemonSet
        ## When mode is Deployment, specify the desired Agent replicas
        # replicas: 2
```

Available strategies are the following:
- `DaemonSet`: runs on every node
- `Deployment`: useful to decrease the resource footprint in certain workloads cluster,
  it allows customising also the amount of deployed replicas via the field
  `tenantcontrolplane.spec.addons.konnectivity.agent.replicas`. 

## Agent keepalive

Kamaji starts the Konnectivity Agent with `--keepalive-time=60s`:
after 60 seconds without activity on its tunnel, the agent pings the Konnectivity server.
The upstream default is one hour, and with it a tunnel whose network path changed,
for example after a worker node was live-migrated or a NAT or load balancer in the path dropped its connection state,
can stay in use while it is dead: `kubectl logs`, `exec` and `port-forward` then fail with `No agent available`
until the agent reconnects.
The ping lets the agent detect the dead tunnel and reconnect, instead of waiting for up to an hour.

You can tune the value through `tenantcontrolplane.spec.addons.konnectivity.agent.extraArgs`,
which takes precedence over the arguments set by Kamaji:

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: konnectivity-example
spec:
  addons:
    konnectivity:
      agent:
        extraArgs:
          - --keepalive-time=30s
```

- Raise it, for example `--keepalive-time=5m`, to reduce keepalive traffic on clusters with many nodes;
  `--keepalive-time=1h` restores the upstream default.
- Lower it to detect a broken tunnel faster.
  Do not go below `30s`: the Konnectivity server enforces that minimum interval between pings,
  and closes the connection of an agent that pings more often.

Changing the value changes the agent's Pod template, so the agent Pods are rolled out again.

---

By integrating Konnectivity as a core feature, Kamaji ensures that your Tenant Clusters can operate reliably and securely across any network topology,
making it easier to build and manage distributed Kubernetes environments at scale.

## Version compatibility between API Server and Konnectivity

In recent Kubernetes releases, Konnectivity has aligned its versioning with the Kubernetes API Server.

This means that for example:
- Kubernetes v1.35.0 pairs with Konnectivity v0.34.0
- Kubernetes v1.33.0 pairs with Konnectivity v0.33.0
- and so on for each Kubernetes version

Within Kamaji, this version matching happens automatically.

The field `TenantControlPlane.spec.addons.konnectivity` determines the proper Konnectivity version for both the server and the agent,
ensuring compatibility with the tenant control plane's API Server version.

!!! warning "Konnectivity images could not be available!"
    For the most recent Kubernetes releases, the corresponding Konnectivity image artifacts _may not yet be built and published_ by the upstream community.
    In these cases, you may need to override the automatic pairing and configure a previous Konnectivity version that is available.

You can still have a version skew between the Kubernetes API Server for the given Tenant Control Plane, and the Konnectivity components.

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: konnectivity
  namespace: default
spec:
  addons:
    coreDNS: {}
    konnectivity:
      agent:
        hostNetwork: false
        image: registry.k8s.io/kas-network-proxy/proxy-agent
        mode: DaemonSet
        tolerations:
        - key: CriticalAddonsOnly
          operator: Exists
        version: v0.33.0
      server:
        image: registry.k8s.io/kas-network-proxy/proxy-server
        port: 8132
        version: v0.33.0
    kubeProxy: {}
  controlPlane:
    deployment:
      replicas: 2
    service:
      serviceType: LoadBalancer
  dataStore: etcd-kamaji-etcd
  kubernetes:
    kubelet:
      cgroupfs: systemd
      preferredAddressTypes:
      - InternalIP
      - ExternalIP
      - Hostname
    version: v1.34.0
  networkProfile:
    clusterDomain: cluster.local
    dnsServiceIPs:
    - 10.96.0.10
    podCidr: 10.244.0.0/16
    port: 6443
    serviceCidr: 10.96.0.0/16
```
