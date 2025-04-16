# Tenant Worker Nodes

While Kamaji innovates in how control planes are managed, Tenant Worker Nodes remain true to their Kubernetes roots: they are regular virtual machines or bare metal servers that run your workloads. What makes them special in Kamaji's architecture is how they integrate with the containerized control planes and how they can be managed at scale across diverse infrastructure environments.

## Understanding Worker Nodes in Kamaji

In a Kamaji managed cluster, worker nodes connect to their Tenant Control Plane just as they would in a traditional Kubernetes setup. The key difference is that the control plane they're connecting to runs as pods within the Management Cluster, rather than on dedicated machines. This architectural choice maintains compatibility with existing tools and workflows while enabling more efficient resource utilization.

Each worker node belongs to exactly one Tenant Cluster and runs only that tenant's workloads. This clear separation ensures strong isolation between different tenants' applications and data, making Kamaji suitable for multi-tenant environments.

## Infrastructure Flexibility

Your worker nodes can run:

- On bare metal servers in a data center
- As virtual machines in private clouds
- On public cloud instances
- At edge locations
- In hybrid or multi-cloud configurations

This flexibility allows you to place workloads where they make the most sense for your use case, whether that's close to users, near data sources, or in specific regulatory environments.

## Lifecycle Management Options

Kamaji supports multiple approaches to managing worker node lifecycles:

### Manual Management
For simple setups or specific requirements, you can join worker nodes to their Tenant Clusters using standard `kubeadm` commands. This process is familiar to Kubernetes administrators and works just as it would with traditionally deployed clusters.

!!! tip "yaki"
    See [yaki](https://goyaki.clastix.io/) script, which you could modify for your preferred operating system and version. The provided script is just a facility: it assumes all worker nodes are running `Ubuntu`. Make sure to adapt the script if you're using a different OS distribution.

### Automation Tools
You can use standard infrastructure automation tools to manage worker nodes:

- Terraform for infrastructure provisioning
- Ansible for configuration management


### Cluster API Integration
For more sophisticated automation, Kamaji provides a [Cluster API Control Plane Provider](https://github.com/clastix/cluster-api-control-plane-provider-kamaji).

This integration enables:

- Declarative management of both tenant control planes and tenant worker nodes
- Automated scaling and updates
- Integration with infrastructure providers for major cloud platforms
- Consistent management across different environments

---

Kamaji's approach to worker nodes combines the familiarity of traditional Kubernetes with the flexibility to run anywhere and the ability to manage at scale. Whether you're building a private cloud platform, offering Kubernetes as a service, or managing edge computing infrastructure, Kamaji provides the tools and patterns you need.

