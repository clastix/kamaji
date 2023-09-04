# CNCF Conformance
For organizations using Kubernetes, conformance enables interoperability, consistency, and confirmability between Kubernetes installations. The Cloud Computing Native Foundation - CNCF - provides the [Certified Kubernetes Conformance Program](https://www.cncf.io/certification/software-conformance/).

The standard set of conformance tests is currently those defined by the `[Conformance]` tag in the
[kubernetes e2e](https://github.com/kubernetes/kubernetes/tree/master/test/e2e) suite.

All the _“Tenant Clusters”_ built with Kamaji are CNCF conformant:

- [v1.23](https://github.com/cncf/k8s-conformance/pull/2194)
- [v1.24](https://github.com/cncf/k8s-conformance/pull/2193)
- [v1.25](https://github.com/cncf/k8s-conformance/pull/2188)
- [v1.26](https://github.com/cncf/k8s-conformance/pull/2787)
- [v1.27](https://github.com/cncf/k8s-conformance/pull/2786)
- [v1.28](https://github.com/cncf/k8s-conformance/pull/2785)

<p align="left" style="padding: 6px 6px">
  <img src="https://raw.githubusercontent.com/cncf/artwork/master/projects/kubernetes/certified-kubernetes/versionless/color/certified-kubernetes-color.png" width="100" />
</p>

## Running the conformance tests

The standard tool for running CNCF conformance tests is [Sonobuoy](https://github.com/vmware-tanzu/sonobuoy). Sonobuoy is
regularly built and kept up to date to execute against all currently supported versions of kubernetes.

Download a [binary release](https://github.com/vmware-tanzu/sonobuoy/releases) of the CLI.

Make sure to access your Tenant Cluster:

```
export KUBECONFIG=tenant.kubeconfig
```

Deploy a Sonobuoy pod to your Tenant Cluster with:

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



