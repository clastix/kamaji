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

## Multi-tenant MySQL-MariaDB cluster

> This assumes you already have a running Kubernetes cluster and kubeconfig.

Read [this](kine/mysql/README.md) in order to know more about.
