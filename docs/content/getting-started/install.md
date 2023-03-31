# Install Kamaji

## Quickstart

### Pre-requisites

- [Helm](https://helm.sh/docs/intro/install/)
- Kubernetes cluster

### Install cert-manager

```shell
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install \
  cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version v1.11.0 \
  --set installCRDs=true
```

### Install Kamaji with default datastore

```
helm repo add clastix https://clastix.github.io/charts
helm repo update
helm install kamaji clastix/kamaji -n kamaji-system --create-namespace
```

Now you're ready to go with Kamaji!

Please follow the documentation to start playing with it.

