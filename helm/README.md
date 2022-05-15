# Deploy with Helm

## Pre-requisites

1. Deploy a [multi-tenant Etcd cluster](https://github.com/clastix/kamaji-internal/blob/master/deploy/getting-started-with-kamaji.md#setup-internal-multi-tenant-etcd)

```
make -C ../deploy/etcd
```

## Install

```
helm upgrade --install --namespace kamaji-system --create-namespace kamaji ./kamaji
```
