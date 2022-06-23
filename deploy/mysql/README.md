# MySQL as Kubernetes Storage

Kamaji offers the possibility of having a different storage system than `ETCD` thanks to [kine](https://github.com/k3s-io/kine). One of the implementations is [MySQL](https://www.mysql.com/).

Kamaji project is developed using [kind](https://kind.sigs.k8s.io), therefore, MySQL (or [MariaDB](https://mariadb.org/) in this case) will be deployed into the local kubernetes cluster in order to be used as storage for the tenants.

There is a Makefile to help with the process:

* **Full Installation**

```bash
$ make mariadb
```

This action will perform all the necessary stuffs to have MariaDB as kubernetes storage backend using kine.

* **Certificate creation**

```bash
$ make mariadb-certificates
```

Communication between kine and the backend is encrypted, therefore, some certificates must be created.

* **Secret Deployment**

```bash
$ make mariadb-secrets
```

Previous certificates and MySQL configuration have to be available in order to be used. They will be under the secret `kamaji-system:mysql-config`.

* **Deployment**

```bash
$ make mariadb-deployment
```

* **Uninstall Everything**

```bash
$ make destroy
```