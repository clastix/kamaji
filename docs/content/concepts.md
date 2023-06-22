# Concepts

Kamaji is a Kubernetes Operator. It turns any Kubernetes cluster into an _“admin cluster”_ to orchestrate other Kubernetes clusters called _“tenant clusters”_.

These are requirements of the design behind Kamaji:

- Communication between the _“admin cluster”_ and a _“tenant cluster”_ is unidirectional. The _“admin cluster”_ manages a _“tenant cluster”_, but a _“tenant cluster”_ has no awareness of the _“admin cluster”_.
- Communication between different _“tenant clusters”_ is not allowed.
- The worker nodes of tenant should not run anything beyond tenant's workloads.

Goals and scope may vary as the project evolves.

## Tenant Control Plane
Kamaji is special because the Control Planes of the _“tenant cluster”_ are regular pods running in a namespace of the _“admin cluster”_ instead of a dedicated set of Virtual Machines. This solution makes running Control Planes at scale cheaper and easier to deploy and operate. The Tenant Control Plane components are packaged in the same way they are running in bare metal or virtual nodes. We leverage the `kubeadm` code to set up the control plane components as they were running on their own server. The unchanged images of upstream `kube-apiserver`, `kube-scheduler`, and `kube-controller-manager` are used.

High Availability and rolling updates of the Tenant Control Plane pods are provided by a regular Deployment. Autoscaling based on the metrics is available. A Service is used to espose the Tenant Control Plane outside of the _“admin cluster”_. The `LoadBalancer` service type is used, `NodePort` and `ClusterIP` are other viable options, depending on the case.

Kamaji offers a [Custom Resource Definition](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) to provide a declarative approach of managing a Tenant Control Plane. This *CRD* is called `TenantControlPlane`, or `tcp` in short.

All the _“tenant clusters”_ built with Kamaji are fully compliant CNCF Kubernetes clusters and are compatible with the standard Kubernetes toolchains everybody knows and loves. See [CNCF compliance](reference/conformance.md).

## Tenant worker nodes

And what about the tenant worker nodes?
They are just _"worker nodes"_, i.e. regular virtual or bare metal machines, connecting to the APIs server of the Tenant Control Plane.
Kamaji's goal is to manage the lifecycle of hundreds of these _“tenant clusters”_, not only one, so how to add another tenant cluster to Kamaji?
As you could expect, you have just deploys a new Tenant Control Plane in one of the _“admin cluster”_ namespace, and then joins the tenant worker nodes to it.

A [Cluster API ControlPlane provider](https://github.com/clastix/cluster-api-control-plane-provider-kamaji) has been released, allowing to offer a Cluster API-native declarative lifecycle, by automating the worker nodes join.

## Datastores
Putting the Tenant Control Plane in a pod is the easiest part. Also, we have to make sure each tenant cluster saves the state to be able to store and retrieve data. As we can deploy a Kubernetes cluster with an external `etcd` cluster, we explored this option for the Tenant Control Planes. On the admin cluster, you can deploy one or multi-tenant `etcd` to save the state of multiple tenant clusters. Kamaji offers a Custom Resource Definition called `DataStore` to provide a declarative approach of managing multiple datastores. By sharing the datastore between multiple tenants, the resiliency is still guaranteed and the pods' count remains under control, so it solves the main goal of resiliency and costs optimization. The trade-off here is that you have to operate external datastores, in addition to `etcd` of the _“admin cluster”_ and manage the access to be sure that each _“tenant cluster”_ uses only its data.

### Other storage drivers
Kamaji offers the option of using a more capable datastore than `etcd` to save the state of multiple tenants' clusters. Thanks to the native [kine](https://github.com/k3s-io/kine) integration, you can run _MySQL_ or _PostgreSQL_ compatible databases as datastore for _“tenant clusters”_.

### Pooling
By default, Kamaji is expecting to persist all the _“tenant clusters”_ data in a unique datastore that could be backed by different drivers. However, you can pick a different datastore for a specific set of _“tenant clusters”_ that could have different resources assigned or a different tiering. Pooling of multiple datastore is an option you can leverage for a very large set of _“tenant clusters”_ so you can distribute the load properly. As future improvements, we have a _datastore scheduler_ feature in roadmap so that Kamaji itself can assign automatically a _“tenant cluster”_ to the best datastore in the pool.

### Migration
In order to simplify Day2 Operations and reduce the operational burden, Kamaji provides the capability to live migrate data from a datastore to another one of the same driver without manual and error prone backup and restore operations.

> Currently, live data migration is only available between datastores having the same driver.

## Konnectivity

In addition to the standard control plane containers, Kamaji creates an instance of [konnectivity-server](https://kubernetes.io/docs/concepts/architecture/control-plane-node-communication/) running as sidecar container in the `tcp` pod and exposed on port `8132` of the `tcp` service.

This is required when the tenant worker nodes are not reachable from the `tcp` pods. The Konnectivity service consists of two parts: the Konnectivity server in the tenant control plane pod and the Konnectivity agents running on the tenant worker nodes.

After worker nodes joined the tenant control plane, the Konnectivity agents initiate connections to the Konnectivity server and maintain the network connections. After enabling the Konnectivity service, all control plane to worker nodes traffic goes through these connections.

> In Kamaji, Konnectivity is enabled by default and can be disabled when not required.

