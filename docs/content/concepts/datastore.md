# Datastore

A critical part of any Kubernetes control plane is its datastore, the system that persists the cluster’s state, configuration, and operational data. In Kamaji, this requirement is addressed with flexibility and scalability in mind, allowing you to choose the best storage backend for your needs and to manage many clusters efficiently.

Kamaji’s architecture decouples the control plane from its underlying datastore. Instead of each Tenant Cluster running its own dedicated datastore instance, Kamaji enables you to share datastores across multiple Tenant Clusters, or assign a dedicated datastore to each Tenant Cluster where needed. This approach optimizes resource usage, simplifies operations, and supports a variety of backend technologies.

## Supported Datastore Backends

Kamaji supports several options for persisting Tenant Cluster state:

- **etcd:**  
  The default and most widely used Kubernetes datastore. You can deploy one or more etcd clusters in the Management Cluster and assign them to Tenant Control Planes as needed.

- **SQL Databases:**  
  For environments where etcd is not ideal, Kamaji integrates with [kine](https://github.com/k3s-io/kine), allowing you to use MySQL or PostgreSQL-compatible databases as the backend for Tenant Clusters.

!!! info "NATS"
    The support of [NATS](https://nats.io/) is still experimental, mostly because multi-tenancy is not (yet) supported in NATS.

## Declarative Management

Datastores are managed declaratively using the `DataStore` Custom Resource Definition (CRD). This makes it easy to define, configure, and assign datastores to Tenant Control Planes, and fits naturally into GitOps and Infrastructure as Code workflows.

## Pooling and Scalability

By default, Kamaji can persist all Tenant Clusters’ data in a single datastore, but you can also create pools of datastores and assign clusters based on resource requirements, performance needs, or organizational policies. This pooling capability is especially useful for large-scale environments, where distributing the load across multiple datastores ensures resilience and scalability.

Kamaji’s roadmap includes a datastore scheduler, which will automatically assign new Tenant Clusters to the most appropriate datastore in the pool, further reducing operational overhead.

## Live Migration

Operational needs change over time, and Kamaji makes it easy to adapt. You can live-migrate a Tenant Cluster’s data from one datastore to another, as long as they use the same backend driver, without manual backup and restore steps. This feature simplifies Day 2 operations and helps you optimize your infrastructure as your requirements evolve.

!!! info "Datastore Migration"
    Currently, live data migration is only available between datastores having the same driver.

