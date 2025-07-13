# Konnectivity

In traditional Kubernetes deployments, the control plane components need to communicate directly with worker nodes for various operations
like executing commands in pods, retrieving logs, or managing port forwards.
However, in many real-world environments, especially those spanning multiple networks or cloud providers,
direct communication isn't always possible or desirable. This is where Konnectivity comes in.

## Understanding Konnectivity in Kamaji

Kamaji integrates [Konnectivity](https://kubernetes.io/docs/concepts/architecture/control-plane-node-communication/) as a core component of its architecture.
Each Tenant Control Plane pod includes a konnectivity-server running as a sidecar container,
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

---

By integrating Konnectivity as a core feature, Kamaji ensures that your Tenant Clusters can operate reliably and securely across any network topology,
making it easier to build and manage distributed Kubernetes environments at scale.
