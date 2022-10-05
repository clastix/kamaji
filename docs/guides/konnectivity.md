# Set up Konnectivity service

In addition to the standard control plane containers, Kamaji creates an instance of [konnectivity-server](https://kubernetes.io/docs/concepts/architecture/control-plane-node-communication/) running as sidecar container in the `tcp` pod and exposed on port `8132` of the `tcp` service.

This is required when the tenant worker nodes are not reachable from the `tcp` pods. The Konnectivity service consists of two parts: the Konnectivity server in the tenant control plane pod and the Konnectivity agents running on the tenant worker nodes.

After worker nodes joined the tenant control plane, the Konnectivity agents initiate connections to the Konnectivity server and maintain the network connections. After enabling the Konnectivity service, all control plane to worker nodes traffic goes through these connections.

> In Kamaji, Konnectivity is enabled by default and can be disabled when not required.
