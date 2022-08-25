# PostgreSQL as Kubernetes Storage

Kamaji offers the possibility of having a different storage system than `etcd` thanks to [kine](https://github.com/k3s-io/kine).
One of the implementations is [PostgreSQL](https://www.postgresql.org/).

Kamaji project is developed using [kind](https://kind.sigs.k8s.io), therefore, a PostgreSQL instance must be deployed in advance into the local kubernetes cluster in order to be used as storage for the tenants.
For the sake of simplicity, the [cloudnative-pg](https://cloudnative-pg.io/) Operator will be used to simplify the setup of it.

There is a Makefile to help with the process:

## Setup

```bash
$ make postgresql
```

This target will install the `cloudnative-pg`, creating the PostgreSQL instance in the Kamaji Namespace, along with the generation of the required Secret resource for the kine integration.

This action is idempotent and doesn't overwrite values if they already exist.

```shell
namespace/cnpg-system unchanged
customresourcedefinition.apiextensions.k8s.io/backups.postgresql.cnpg.io configured
customresourcedefinition.apiextensions.k8s.io/clusters.postgresql.cnpg.io configured
customresourcedefinition.apiextensions.k8s.io/poolers.postgresql.cnpg.io configured
customresourcedefinition.apiextensions.k8s.io/scheduledbackups.postgresql.cnpg.io configured
serviceaccount/cnpg-manager unchanged
clusterrole.rbac.authorization.k8s.io/cnpg-manager configured
clusterrolebinding.rbac.authorization.k8s.io/cnpg-manager-rolebinding unchanged
configmap/cnpg-default-monitoring unchanged
service/cnpg-webhook-service unchanged
deployment.apps/cnpg-controller-manager unchanged
mutatingwebhookconfiguration.admissionregistration.k8s.io/cnpg-mutating-webhook-configuration configured
validatingwebhookconfiguration.admissionregistration.k8s.io/cnpg-validating-webhook-configuration configured
deployment "cnpg-controller-manager" successfully rolled out
cluster.postgresql.cnpg.io/postgresql unchanged
secret/postgres-root-cert created
```

## Operator setup

```bash
$ make cnpg-setup
```

This target will apply all the required manifests with the `cloudnative-pg` CRD, and required RBAC, and Deployment.

Release [v1.16.0](https://github.com/cloudnative-pg/cloudnative-pg/releases/tag/v1.16.0) has been tested successfully.

## SSL certificate Secret generation

```bash
$ make postgresql-secret
```

This target will download locally the `kubectl-cnpg` utility to generate an SSL certificate required to secure the connection to the PostgreSQL instance.

## Certificate generation

```bash
$ make postgresql-secret
```

Generate the Certificate required to connect to the DataStore.

## Teardown

```bash
$ make postgresql-destroy
```

This will lead to the deletion of the `cloudnative-pg` Operator, along with any instance, and related secrets.

This action is idempotent.
