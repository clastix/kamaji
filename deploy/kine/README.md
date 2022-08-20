# Kine integration

[kine](https://github.com/k3s-io/kine) is an `etcd` shim that allows to use a different datastore for your Kubernetes cluster.

Kamaji actually allows to run a shared datastore using different MySQL and PostgreSQL schemas per Tenant.
This can help in overcoming the `etcd` limitation regarding scalability and cluster size, as well with HA and replication.

## Kamaji additional CLI flags

Once a compatible database is running, we need to provide information about it to Kamaji by using the following flags:

```
--etcd-storage-type={kine-mysql,kine-postgresql}
--kine-host=<database host>
--kine-port=<database port>
--kine-secret-name=<secret name>
--kine-secret-namespace=<secret namespace>
```

## Kine Secret

The Kine Secret must be configured as follows:

```yaml
apiVersion: v1
data:
  ca.crt: "content of the Certificate Authority for SSL connection"
  password: "password of the super user"
  server.crt: "content of the certificate for SSL connection"
  server.key: "content of the private key for SSL connection"
  username: "username of the super user"
kind: Secret
metadata:
  name: kine-secret
  namespace: kamaji-system
type: kamaji.clastix.io/kine
```

> Please, pay attention to the type `kamaji.clastix.io/kine`: this check is enforced at the code level to ensure the required data is provided.

> Actually, the `kine` integration expects a secured connection to the database since the sensitivity data of the Tenant.

## Drivers

Further details on the setup for each driver are available here:
- [MySQL/MariaDB](../deploy/kine/mysql/README.md)
- [PostgreSQL](../deploy/kine/postgresql/README.md)