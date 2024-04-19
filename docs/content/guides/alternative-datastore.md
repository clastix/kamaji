# Use Alternative Datastores

Kamaji offers the possibility of having a different storage system than `etcd` thanks to [kine](https://github.com/k3s-io/kine) integration. One of the implementations is [PostgreSQL](https://www.postgresql.org/).

## Install the datastore

On the Management Cluster, install one of the alternative supported datastore:

- **MySQL** install it with command:

    `$ make -C deploy/kine/mysql mariadb`

- **PostgreSQL** install it with command:

    `$ make -C deploy/kine/postgresql postgresql`

- **NATS**

*Note: NATS SUPPORT IS EXPERIMENTAL: Currently multi-tenancy is NOT supported when using NATS as an alternative datastore*

Currently, only username/password auth is supported.

```bash
cat << EOF > values-nats.yaml
config:
  merge:
    accounts:
      private:
        jetstream: enabled
        users:
        - {user: admin, password: "password", permissions: {subscribe: [">"], publish: [">"]}}
  cluster:
    enabled: no
  nats:
    tls:
      enabled: true
      secretName: nats-config
      cert: server.crt
      key: server.key
  jetstream:
    enabled: true
    fileStore:
      pvc:
        size: 32Mi

EOF
```

```bash
  repo add nats https://nats-io.github.io/k8s/helm/charts/

  helm install nats/nats \
  -f values-nats.yaml
  --namespace nats-system \
  --create-namespace
```


## Install Cert Manager

As prerequisite for Kamaji, install the Cert Manager

```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install \
  cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version v1.11.0 \
  --set installCRDs=true
```

## Install Kamaji

Use Helm to install the Kamaji Operator and make sure it uses a datastore with the proper driver `datastore.driver=<MySQL|PostgreSQL|NATS>`.

For example, with a PostreSQL datastore installed:

```bash
helm install kamaji charts/kamaji -n kamaji-system --create-namespace \
  --set etcd.deploy=false \
  --set datastore.driver=PostgreSQL \
  --set datastore.endpoints[0]=postgres-default-rw.kamaji-system.svc:5432 \
  --set datastore.basicAuth.usernameSecret.name=postgres-default-superuser \
  --set datastore.basicAuth.usernameSecret.namespace=kamaji-system \
  --set datastore.basicAuth.usernameSecret.keyPath=username \
  --set datastore.basicAuth.passwordSecret.name=postgres-default-superuser \
  --set datastore.basicAuth.passwordSecret.namespace=kamaji-system \
  --set datastore.basicAuth.passwordSecret.keyPath=password \
  --set datastore.tlsConfig.certificateAuthority.certificate.name=postgres-default-ca \
  --set datastore.tlsConfig.certificateAuthority.certificate.namespace=kamaji-system \
  --set datastore.tlsConfig.certificateAuthority.certificate.keyPath=ca.crt \
  --set datastore.tlsConfig.certificateAuthority.privateKey.name=postgres-default-ca \
  --set datastore.tlsConfig.certificateAuthority.privateKey.namespace=kamaji-system \
  --set datastore.tlsConfig.certificateAuthority.privateKey.keyPath=ca.key \
  --set datastore.tlsConfig.clientCertificate.certificate.name=postgres-default-root-cert \
  --set datastore.tlsConfig.clientCertificate.certificate.namespace=kamaji-system \
  --set datastore.tlsConfig.clientCertificate.certificate.keyPath=tls.crt \
  --set datastore.tlsConfig.clientCertificate.privateKey.name=postgres-default-root-cert \
  --set datastore.tlsConfig.clientCertificate.privateKey.namespace=kamaji-system \
  --set datastore.tlsConfig.clientCertificate.privateKey.keyPath=tls.key
```

Once installed, you will able to create Tenant Control Planes using an alternative datastore.