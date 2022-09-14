# CNCF Conformance
For organizations using Kubernetes, conformance enables interoperability, consistency, and confirmability between Kubernetes installations. The Cloud Computing Native Foundation - CNCF - provides the [Certified Kubernetes Conformance Program](https://www.cncf.io/certification/software-conformance/). All the _“tenant clusters”_ built with Kamaji are CNCF conformant.

The standard set of conformance tests is currently those defined by the `[Conformance]` tag in the
[kubernetes e2e](https://github.com/kubernetes/kubernetes/tree/master/test/e2e) suite.

## Running the conformance tests

The standard tool for running CNCF conformance tests is [Sonobuoy](https://github.com/vmware-tanzu/sonobuoy). Sonobuoy is
regularly built and kept up to date to execute against all currently supported versions of kubernetes.

Download a [binary release](https://github.com/vmware-tanzu/sonobuoy/releases) of the CLI.

Make sure to access your tenant cluster:

```
export KUBECONFIG=tenant.kubeconfig
```

Deploy a Sonobuoy pod to your tenant cluster with:

```
sonobuoy run --mode=certified-conformance
```

> You can run the command synchronously by adding the flag `--wait` but be aware that running the conformance tests can take an hour or more.

View actively running pods:

```
sonobuoy status
```

To inspect the logs:

```
sonobuoy logs -f
```

Once `sonobuoy status` shows the run as `completed`, copy the output directory from the main Sonobuoy pod to a local directory:

```
outfile=$(sonobuoy retrieve)
```

This copies a single `.tar.gz` snapshot from the Sonobuoy pod into your local
`.` directory. Extract the contents into `./results` with:

```
mkdir ./results; tar xzf $outfile -C ./results
```

To clean up Kubernetes objects created by Sonobuoy, run:

```
sonobuoy delete
```



