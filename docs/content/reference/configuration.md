## Configuration

Currently, **Kamaji** allows customization using CLI flags for the `manager` subcommand.

Available flags are the following:

```
      --datastore string                   The default DataStore that should be used by Kamaji to setup the required storage (default "etcd")
      --health-probe-bind-address string   The address the probe endpoint binds to. (default ":8081")
  -h, --help                               help for manager
      --kine-image string                  Container image along with tag to use for the Kine sidecar container (used only if etcd-storage-type is set to one of kine strategies) (default "rancher/kine:v0.9.2-amd64")
      --leader-elect                       Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager. (default true)
      --metrics-bind-address string        The address the metric endpoint binds to. (default ":8080")
      --tmp-directory string               Directory which will be used to work with temporary files. (default "/tmp/kamaji")
      --zap-devel                          Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error) (default true)
      --zap-encoder encoder                Zap log encoding (one of 'json' or 'console')
      --zap-log-level level                Zap Level to configure the verbosity of logging. Can be one of 'debug', 'info', 'error', or any integer value > 0 which corresponds to custom debug levels of increasing verbosity
      --zap-stacktrace-level level         Zap Level at and above which stacktraces are captured (one of 'info', 'error', 'panic').
      --zap-time-encoding time-encoding    Zap time encoding (one of 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano'). Defaults to 'epoch'.
```
