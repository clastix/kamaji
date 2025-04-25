# Cluster APIs Support

The [Cluster API](https://github.com/kubernetes-sigs/cluster-api) brings declarative, Kubernetes-style APIs to the creation, configuration, and management of Kubernetes clusters. If you're not familiar with the Cluster API project, you can learn more from the [official documentation](https://cluster-api.sigs.k8s.io/).

Users can utilize Kamaji in two distinct ways:

* **Standalone:** Kamaji can be used as a standalone Kubernetes Operator installed in the Management Cluster to manage multiple Tenant Control Planes. Worker nodes of Tenant Clusters can join any infrastructure, whether it be cloud, data-center, or edge, using various automation tools such as _Ansible_, _Terraform_, or even manually with any script calling `kubeadm`. See [yaki](https://goyaki.clastix.io/) as an example.

* **Cluster API Provider:** Kamaji can be used as a [Cluster API Control Plane Provider](https://cluster-api.sigs.k8s.io/reference/providers#control-plane) to manage multiple Tenant Control Planes across various infrastructures. Kamaji offers seamless integration with the most popular [Cluster API Infrastructure Providers](https://cluster-api.sigs.k8s.io/reference/providers#infrastructure).

!!! tip "Control Plane and Infrastructure Decoupling"
    Kamaji decouples the Control Plane from the infrastructure, allowing the Kamaji Management Cluster to reside on a different infrastructure or cloud provider than the Tenant worker machines, as long as network reachability is ensured. This flexibility enables mixing and matching infrastructure providers, such as hosting the Management Cluster on a public cloud while deploying Tenant worker machines on private data centers, edge environments, or other clouds.

Check the currently supported infrastructure providers and the roadmap on the related [repository](https://github.com/clastix/cluster-api-control-plane-provider-kamaji).