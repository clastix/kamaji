## Configuration

Currently **Kamaji** supports (in this order):

* CLI flags
* Environment variables
* Configuration files

By default **Kamaji** search for the configuration file (`kamaji.yaml`) and uses parameters found inside of it. In case some environment variable are passed, this will override configuration file parameters. In the end, if also a CLI flag is passed, this will override both env vars and config file as well.

This is easily explained in this way:

`cli-flags` > `env-vars` > `config-files`

Available flags are the following:

```
--config-file string                 Configuration file alternative. (default "./kamaji.yaml")
--etcd-ca-secret-name                Name of the secret which contains CA's certificate and private key. (default: "etcd-certs")
--etcd-ca-secret-namespace           Namespace of the secret which contains CA's certificate and private key. (default: "kamaji")
--etcd-client-secret-name            Name of the secret which contains ETCD client certificates. (default: "root-client-certs")
--etcd-client-secret-namespace       Name of the namespace where the secret which contains ETCD client certificates is. (default: "kamaji")
--etcd-compaction-interval           ETCD Compaction interval (i.e. "5m0s"). (default: "0" (disabled))
--etcd-endpoints                     Comma-separated list with ETCD endpoints (i.e. https://etcd-0.etcd.kamaji.svc.cluster.local,https://etcd-1.etcd.kamaji.svc.cluster.local,https://etcd-2.etcd.kamaji.svc.cluster.local)
--etcd-storage-type                  ETCD Storage type (i.e. "etcd", "kine-mysql"). (default: "etcd")
--health-probe-bind-address string   The address the probe endpoint binds to. (default ":8081")
--kine-mysql-host                    Host where MySQL is running (default: "localhost")
--kine-mysql-port int                Port where MySQL is running (default: 3306)
--kine-mysql-secret-name             Name of the secret where the necessary configuration and certificates are. (default: "mysql-config")
--kine-mysql-secret-name             Name of the namespace of the secret where the necessary configuration and certificates are. (default: "kamaji-system")
--kubeconfig string                  Paths to a kubeconfig. Only required if out-of-cluster.
--leader-elect                       Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.
--metrics-bind-address string        The address the metric endpoint binds to. (default ":8080")
--tmp-directory                      Directory which will be used to work with temporary files. (default "/tmp/kamaji")
--zap-devel                          Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error) (default true)
--zap-encoder encoder                Zap log encoding (one of 'json' or 'console')
--zap-log-level level                Zap Level to configure the verbosity of logging. Can be one of 'debug', 'info', 'error', or any integer value > 0 which corresponds to custom debug levels of increasing verbosity
--zap-stacktrace-level level         Zap Level at and above which stacktraces are captured (one of 'info', 'error', 'panic').
```

Available environment variables are:

| Environment variable               | Description                                                  |
| ---------------------------------- | ------------------------------------------------------------ |
| `KAMAJI_ETCD_CA_SECRET_NAME`      | Name of the secret which contains CA's certificate and private key. (default: "etcd-certs")  |
| `KAMAJI_ETCD_CA_SECRET_NAMESPACE`      | Namespace of the secret which contains CA's certificate and private key. (default: "kamaji")  |
| `KAMAJI_ETCD_CLIENT_SECRET_NAME`      | Name of the secret which contains ETCD client certificates. (default: "root-client-certs")  |
| `KAMAJI_ETCD_CLIENT_SECRET_NAMESPACE`      | Name of the namespace where the secret which contains ETCD client certificates is. (default: "kamaji")  |
| `KAMAJI_ETCD_COMPACTION_INTERVAL`      | ETCD Compaction interval (i.e. "5m0s"). (default: "0" (disabled))  |
| `KAMAJI_ETCD_ENDPOINTS`      | Comma-separated list with ETCD endpoints (i.e. etcd-server-1:2379,etcd-server-2:2379). (default: "etcd-server:2379")  |
| `KAMAJI_ETCD_STORAGE_TYPE`      | ETCD Storage type (i.e. "etcd", "kine-mysql"). (default: "etcd")  |
| `KAMAJI_ETCD_SERVERS`      | Comma-separated list with ETCD servers (i.e. etcd-0.etcd.kamaji.svc.cluster.local,etcd-1.etcd.kamaji.svc.cluster.local,etcd-2.etcd.kamaji.svc.cluster.local)  |
| `KAMAJI_METRICS_BIND_ADDRESS`      | The address the metric endpoint binds to. (default ":8080")  |
| `KAMAJI_HEALTH_PROBE_BIND_ADDRESS` | The address the probe endpoint binds to. (default ":8081")   |
| `KAMAJI_KINE_MYSQL_HOST` | Host where MySQL is running(default "localhost")   |
| `KAMAJI_KINE_MYSQL_PORT` | Port where MySQL is running (default: 3306)   |
| `KAMAJI_KINE_MYSQL_SECRET_NAME` | Name of the secret where the necessary configuration and certificates are. (default: "mysql-config")   |
| `KAMAJI_KINE_MYSQL_SECRET_NAMESPACE` | Name of the namespace of the secret where the necessary configuration and certificates are. (default: "kamaji-system")   |
| `KAMAJI_LEADER_ELECTION`           | Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager. |
| `KAMAJI_TMP_DIRECTORY`           | Directory which will be used to work with temporary files. (default "/tmp/kamaji") |


## Build and deploy
Clone the repo on your workstation.

```bash
## Install dependencies
$ go mod tidy

## Generate code
$ make generate

## Generate Manifests
$ make manifests

## Install Manifests
$ make install

## Build Docker Image
$ IMG=<image name and tag> make docker-build

## Push Docker Image
$ IMG=<image name and tag> make docker-push

## Deploy Kamaji
$ IMG=<image name and tag> make deploy

## YAML Installation File
$ make yaml-installation-file

```

It will generate a yaml installation file at `config/install.yaml`. It should be customize accordingly.

## Tenant Control Planes

**Kamaji** offers a [CRD](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) to provide a declarative approach of managing tenant control planes. This *CRD* is called `TenantControlPlane`, or `tcp` in short. Use the command `kubectl explain tcp.spec` to understand the fields and their usage.

### Add-ons

**Kamaji** provides optional installations into the deployed tenant control plane through add-ons. Is it possible to enable/disable them through the `tcp` definition.

### Core DNS

```yaml
addons:
    coreDNS: {}
```

### Kube-Proxy

```yaml
addons:
    kubeProxy: {}
```

### Konnectivity

```yaml
addons:
  konnectivity:
    proxyPort: 31132 # mandatory
    version: v0.0.31
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
      limits:
        cpu: 100m
        memory: 128Mi
    serverImage: us.gcr.io/k8s-artifacts-prod/kas-network-proxy/proxy-server
    agentImage: us.gcr.io/k8s-artifacts-prod/kas-network-proxy/proxy-agent
