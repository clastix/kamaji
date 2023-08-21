# Use Alternative Datastores

Kamaji offers the possibility of having a different storage system than `etcd` thanks to [kine](https://github.com/k3s-io/kine) integration. One of the implementations is [PostgreSQL](https://www.postgresql.org/).

## Install the datastore

On the admin cluster, install one of the alternative supported datastore:

- **MySQL** install it with command:

    `$ make -C deploy/kine/mysql mariadb`

- **PostgreSQL** install it with command:

    `$ make -C deploy/kine/postgresql postgresql`

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

Use Helm to install the Kamaji Operator and make sure it uses a datastore with the proper driver `datastore.driver=<MySQL|PostgreSQL>`.

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