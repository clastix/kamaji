# Kamaji

**Kamaji** deploys and operates Kubernetes at scale with a fraction of the operational burden. 

## How it works
Kamaji turns any Kubernetes cluster into an _“admin cluster”_ to orchestrate other Kubernetes clusters called _“tenant clusters”_. Kamaji is special because the Control Plane components are running in a single pod instead of dedicated machines. This solution makes running multiple Control Planes cheaper and easier to deploy and operate. 

<img src="images/architecture.png"  width="600">

View [Concepts](concepts.md) for a deeper understanding of principles behind Kamaji's design.

!!! info "CNCF Compliance"
    All the tenant clusters built with Kamaji are fully compliant [CNCF Certified Kubernetes](https://www.cncf.io/certification/software-conformance/) and are compatible with the standard toolchains everybody knows and loves.

## Getting started

Please refer to the [Getting Started guide](getting-started.md) to deploy a minimal setup of Kamaji.



