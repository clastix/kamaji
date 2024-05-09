# Use Alternative Datastores

Kamaji offers the possibility of having a different storage system than `etcd` thanks to [kine](https://github.com/k3s-io/kine) integration.

## Installing Drivers

The following `make` recipes help you to setup alternative `Datastore` resources.

> The default settings are not production grade:
> the following scripts are just used to test the Kamaji usage of different drivers.

On the Management Cluster, you can use the following commands:

- **MySQL**: `$ make -C deploy/kine/mysql mariadb`

- **PostgreSQL**: `$ make -C deploy/kine/postgresql postgresql`

- **NATS**: `$ make -C deploy/kine/nats nats`

## Defining a default Datastore upon Kamaji installation

Use Helm to install the Kamaji Operator and make sure it uses a datastore with the proper driver `datastore.driver=<MySQL|PostgreSQL|NATS>`.
Please refer to the Chart available values for more information on supported options.

For example, with a PostgreSQL datastore installed:

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

Once installed, you will be able to create Tenant Control Planes using an alternative datastore.

## Defining specific Datastore per Tenant Control Plane

Each `TenantControlPlane` can refer to a specific `Datastore` thanks to the `/spec/dataStore` field.
This allows you to implement your preferred sharding or pooling strategy.

When the said key is omitted, Kamaji will use the default datastore configured with its CLI argument `--datastore`.

## NATS considerations

The NATS support is still experimental, mostly because multi-tenancy is **NOT** supported.

> A `NATS` based DataStore can host one and only one Tenant Control Plane.
> When a `TenantControlPlane` is referring to a NATS `DataStore` already used by another instance,
> reconciliation will fail and blocked.
