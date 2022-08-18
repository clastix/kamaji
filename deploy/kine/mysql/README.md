# MySQL as Kubernetes Storage

Kamaji offers the possibility of having a different storage system than `ETCD` thanks to [kine](https://github.com/k3s-io/kine). One of the implementations is [MySQL](https://www.mysql.com/).

Kamaji project is developed using [kind](https://kind.sigs.k8s.io), therefore, MySQL (or [MariaDB](https://mariadb.org/) in this case) will be deployed into the local kubernetes cluster in order to be used as storage for the tenants.

There is a Makefile to help with the process:

# Setup

Setup of the MySQL/MariaDB backend can be easily issued with a single command.

```bash
$ make mariadb
```

This action will perform all the necessary stuffs to have MariaDB as Kubernetes storage backend using kine.

```shell
rm -rf /home/prometherion/Documents/clastix/kamaji/deploy/mysql/certs && mkdir /home/prometherion/Documents/clastix/kamaji/deploy/mysql/certs
cfssl gencert -initca /home/prometherion/Documents/clastix/kamaji/deploy/mysql/ca-csr.json | cfssljson -bare /home/prometherion/Documents/clastix/kamaji/deploy/mysql/certs/ca
2022/08/18 23:52:56 [INFO] generating a new CA key and certificate from CSR
2022/08/18 23:52:56 [INFO] generate received request
2022/08/18 23:52:56 [INFO] received CSR
2022/08/18 23:52:56 [INFO] generating key: rsa-2048
2022/08/18 23:52:56 [INFO] encoded CSR
2022/08/18 23:52:56 [INFO] signed certificate with serial number 310428005543054656774215122317606431230766314770
cfssl gencert -ca=/home/prometherion/Documents/clastix/kamaji/deploy/mysql/certs/ca.crt -ca-key=/home/prometherion/Documents/clastix/kamaji/deploy/mysql/certs/ca.key \
        -config=/home/prometherion/Documents/clastix/kamaji/deploy/mysql/config.json -profile=server \
        /home/prometherion/Documents/clastix/kamaji/deploy/mysql/server-csr.json | cfssljson -bare /home/prometherion/Documents/clastix/kamaji/deploy/mysql/certs/server
2022/08/18 23:52:56 [INFO] generate received request
2022/08/18 23:52:56 [INFO] received CSR
2022/08/18 23:52:56 [INFO] generating key: rsa-2048
2022/08/18 23:52:56 [INFO] encoded CSR
2022/08/18 23:52:56 [INFO] signed certificate with serial number 582698914718104852311252458344736030793138969927
chmod 644 /home/prometherion/Documents/clastix/kamaji/deploy/mysql/certs/*
secret/mysql-config created
secret/kine-secret created
serviceaccount/mariadb created
service/mariadb created
deployment.apps/mariadb created
persistentvolumeclaim/pvc-mariadb created
```

## Certificate creation

```bash
$ make mariadb-certificates
```

Communication between kine and the backend is encrypted, therefore, a CA and a certificate from it must be created.

## Secret Deployment

```bash
$ make mariadb-secrets
```

Previous certificates and MySQL configuration have to be available in order to be used.
They will be under the secret `kamaji-system:mysql-config`, used by the MySQL/MariaDB instance.

## Kine Secret

```bash
$ make mariadb-kine-secret
```

Organize the required Kine data such as username, password, CA, certificate, and private key to be stored in the Kamaji desired format.

## Deployment

```bash
$ make mariadb-deployment
```

Finally, starts the MySQL/MariaDB installation with all the required settings, such as SSL connection, and configuration.

# Cleanup

```bash
$ make mariadb-destroy
```
