# kamaji

![Version: 0.1.1](https://img.shields.io/badge/Version-0.1.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.1.0](https://img.shields.io/badge/AppVersion-0.1.0-informational?style=flat-square)

A Kubernetes distribution aimed to build and operate a Managed Kubernetes service with a fraction of operational burde.

**Homepage:** <https://github.com/clastix/kamaji-internal/tree/master/helm/kamaji>

### Pre-requisites

Kamaji requires a [multi-tenant etcd cluster](https://github.com/clastix/kamaji-internal/blob/master/deploy/getting-started-with-kamaji.md#setup-internal-multi-tenant-etcd) cluster.
The installation and provisioning processes are already put in place by the Helm Chart starting from v0.1.1 in order to streamline the local test.

> For production use an externally managed etcd is highly recommended, the etcd addon offered by this chart is not considered production-grade.

If you'd like to use an externally managed etcd instance, you can specify the overrides and by setting the value `etcd.deploy=false`.

### Install Kamaji

To install the chart with the release name `kamaji`:

```console
helm upgrade --install --namespace kamaji-system --create-namespace kamaji .
```

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Gonzalo Gabriel Jiménez Fuentes | <iam@mendrugory.com> |  |
| Dario Tranchitella | <dario@tranchitella.eu> |  |
| Massimiliano Giovagnoli | <me@maxgio.it> |  |

## Source Code

* <https://github.com/clastix/kamaji-internal>

## Requirements

Kubernetes: `>=1.18`

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Kubernetes affinity rules to apply to Kamaji controller pods |
| configPath | string | `"./kamaji.yaml"` | Configuration file path alternative. (default "./kamaji.yaml") |
| etcd.compactionInterval | int | `0` | ETCD Compaction interval (e.g. "5m0s"). (default: "0" (disabled)) |
| etcd.deploy | bool | `true` | Install an etcd 3.5 with enabled multi-tenancy along with Kamaji |
| etcd.overrides.caSecret.name | string | `"etcd-certs"` | Name of the secret which contains CA's certificate and private key. (default: "etcd-certs") |
| etcd.overrides.caSecret.namespace | string | `"kamaji-system"` | Namespace of the secret which contains CA's certificate and private key. (default: "kamaji-system") |
| etcd.overrides.clientSecret.name | string | `"root-client-certs"` | Name of the secret which contains ETCD client certificates. (default: "root-client-certs") |
| etcd.overrides.clientSecret.namespace | string | `"kamaji-system"` | Name of the namespace where the secret which contains ETCD client certificates is. (default: "kamaji-system") |
| etcd.overrides.endpoints | string | `"https://etcd-0.etcd.kamaji-system.svc.cluster.local:2379,https://etcd-1.etcd.kamaji-system.svc.cluster.local:2379,https://etcd-2.etcd.kamaji-system.svc.cluster.local:2379"` | (string) Comma-separated list of the endpoints of the etcd cluster's members. |
| etcd.serviceAccount.create | bool | `true` | Create a ServiceAccount, required to install and provision the etcd backing storage (default: true) |
| etcd.serviceAccount.name | string | `""` | Define the ServiceAccount name to use during the setup and provision of the etcd backing storage (default: "") |
| extraArgs | list | `[]` | A list of extra arguments to add to the kamaji controller default ones |
| fullnameOverride | string | `""` |  |
| healthProbeBindAddress | string | `":8081"` | The address the probe endpoint binds to. (default ":8081") |
| image.pullPolicy | string | `"Always"` |  |
| image.repository | string | `"clastix/kamaji"` | The container image of the Kamaji controller. |
| image.tag | string | `"latest"` |  |
| imagePullSecrets | list | `[]` |  |
| livenessProbe | object | `{"httpGet":{"path":"/healthz","port":"healthcheck"},"initialDelaySeconds":15,"periodSeconds":20}` | The livenessProbe for the controller container |
| loggingDevel.enable | bool | `false` | (string) Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error) (default false) |
| metricsBindAddress | string | `":8080"` | (string) The address the metric endpoint binds to. (default ":8080") |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` | Kubernetes node selector rules to schedule Kamaji controller |
| podAnnotations | object | `{}` | The annotations to apply to the Kamaji controller pods. |
| podSecurityContext | object | `{"runAsNonRoot":true}` | The securityContext to apply to the Kamaji controller pods. |
| readinessProbe | object | `{"httpGet":{"path":"/readyz","port":"healthcheck"},"initialDelaySeconds":5,"periodSeconds":10}` | The readinessProbe for the controller container |
| replicaCount | int | `1` | The number of the pod replicas for the Kamaji controller. |
| resources.limits.cpu | string | `"200m"` |  |
| resources.limits.memory | string | `"100Mi"` |  |
| resources.requests.cpu | string | `"100m"` |  |
| resources.requests.memory | string | `"20Mi"` |  |
| securityContext | object | `{"allowPrivilegeEscalation":false}` | The securityContext to apply to the Kamaji controller container only. It does not apply to the Kamaji RBAC proxy container. |
| service.port | int | `8443` |  |
| service.type | string | `"ClusterIP"` |  |
| serviceAccount.annotations | object | `{}` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `"kamaji-controller-manager"` |  |
| temporaryDirectoryPath | string | `"/tmp/kamaji"` | Directory which will be used to work with temporary files. (default "/tmp/kamaji") |
| tolerations | list | `[]` | Kubernetes node taints that the Kamaji controller pods would tolerate |
