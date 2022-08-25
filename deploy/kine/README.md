# Kine integration

[kine](https://github.com/k3s-io/kine) is an `etcd` shim that allows to use a different datastore for your Kubernetes cluster.

Kamaji actually allows to run a shared datastore using different MySQL and PostgreSQL schemas per Tenant.
This can help in overcoming the `etcd` limitation regarding scalability and cluster size, as well with HA and replication.

## Kamaji additional CLI flags

Kamaji read the data store configuration from a cluster-scoped resource named `DataStore`, containing all tha required details to secure a connection using a specific driver. 

- [Example of a `etcd` DataStore](./../../config/samples/kamaji_v1alpha1_datastore_etcd.yaml)
- [Example of a `MySQL` DataStore](./../../config/samples/kamaji_v1alpha1_datastore_mysql.yaml)
- [Example of a `PostgreSQL` DataStore](./../../config/samples/kamaji_v1alpha1_datastore_postgresql.yaml)

Once the datastore is running, and the `DataStore` has been created with the required details, we need to provide information about it to Kamaji by using the following flag and pointing to the resource name:

```
--datastore={.metadata.name}
```

## Drivers

Further details on the setup for each driver are available here:
- [MySQL/MariaDB](../deploy/kine/mysql/README.md)
- [PostgreSQL](../deploy/kine/postgresql/README.md)
