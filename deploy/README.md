# Deploy Kamaji

## Quickstart with KinD

```sh
make -C kind
```

## Multi-tenant etcd cluster

> This assumes you already have a running Kubernetes cluster and kubeconfig.

```sh
make -C etcd
```

## Multi-tenant cluster using Kine

`kine` is an `etcd` shim that allows using different datastore.

Kamaji actually support the following backends:

- [MySQL](kine/mysql/README.md)
- [PostgreSQL](kine/postgresql/README.md)

> This assumes you already have a running Kubernetes cluster and kubeconfig.
