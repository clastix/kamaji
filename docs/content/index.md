# Kamaji

**Kamaji** is the **Kubernetes Control Plane Manager**. It operates Kubernetes at scale with a fraction of the operational burden.

## How it works
Kamaji turns any Kubernetes cluster into a _“Management Cluster”_ to orchestrate other Kubernetes clusters called _“Tenant Clusters”_. Kamaji is special because the Control Plane components are running inside pods instead of dedicated machines. This solution makes running multiple Control Planes cheaper and easier to deploy and operate. 

<img src="images/architecture.png"  width="600">

View [Concepts](concepts.md) for a deeper understanding of principles behind Kamaji's design.

!!! info "CNCF Compliance"
    All the Tenant Clusters built with Kamaji are fully compliant [CNCF Certified Kubernetes](https://www.cncf.io/certification/software-conformance/) and are compatible with the standard toolchains everybody knows and loves.

## Getting started

Please refer to the [Getting Started guide](getting-started.md) to deploy a minimal setup of Kamaji.


## FAQs
Q. What does Kamaji mean?

A. Kamaji is named as the character _Kamajī_ (釜爺, lit. "Boiler Geezer") from the Japanese movie [_Spirited Away_](https://en.wikipedia.org/wiki/Spirited_Away). Kamajī is an elderly man with six, long arms who operates the boiler room of the Bathhouse. The silent professional, whom no one sees, but who gets the hot, fragrant water to all the guests, like our Kamaji provides Kubernetes as a service!

Q. Is Kamaji another Kubernetes distribution yet?

A. No, Kamaji is a Kubernetes Operator you can install on top of any Kubernetes cluster to provide hundreds or thousands of managed Kubernetes clusters as a service. The tenant clusters made with Kamaji are conformant CNCF Kubernetes clusters as we leverage [`kubeadm`](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/).

Q. How is Kamaji different from typical multi-cluster management solutions?

A. Most of the existing multi-cluster management solutions provision specific infrastructure for the control plane, in most cases dedicated machines. Kamaji is special because the control plane of the downstream clusters are regular pods running in the management cluster. This solution makes running control plane at scale cheaper and easier to deploy and operate.

Q. Is it safe to run Kubernetes control plane components in a pod instead of dedicated virtual machines?

A. Yes, the tenant control plane components are packaged in the same way they are running in bare metal or virtual nodes. We leverage the `kubeadm` code to set up the control plane components as they were running on their own server. The unchanged images of upstream `kube-apiserver`, `kube-scheduler`, and `kube-controller-manager` are used, no forks!.

Q. How is Kamaji different from managed Kubernetes services offered by Public Clouds?

A. Kamaji gives you full control over all your Kubernetes infrastructures, offering unparalleled consistency across disparate environments: cloud, data-center, and edge while simplifying and centralizing operations, maintenance, and management tasks. Unlike other Managed Kubernetes services, Kamaji allows you to connect worker nodes from any infrastructure, providing you greater freedom, flexibility, and consistency than public Managed Kubernetes services.

Q. How Kamaji differs from Cluster API?

A. Kamaji and Cluster API complement each other. Kamaji's core idea is having a more efficient control plane management. Cluster API provides a declarative approach to clusters bootstrap and lifecycle management across different environments, cloud providers, and on-premises infrastructures. Thus combined together you get the best of class: Kamaji by simplifying the Control Plane management, Cluster API to abstract from the infrastructure. See supported [CAPI providers](guides/cluster-api.md) by Kamaji.

Q. You already provide a Kubernetes multi-tenancy solution with [Capsule](https://capsule.clastix.io). Why does Kamaji matter?

A. A multi-tenancy solution, like Capsule shares the Kubernetes control plane among all tenants keeping tenant namespaces isolated by policies. While the solution is the right choice by balancing between features and ease of usage, there are cases where a tenant user requires access to the control plane, for example, when a tenant requires to manage CRDs on his own. With Kamaji, you can provide full cluster admin permissions to the tenant.

