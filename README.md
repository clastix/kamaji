# Kamaji - DRAFT

> This project is still in prototyping stage! Please do not use it in production but help us to make it working.

**Kamaji** is a Kubernetes distribution aimed to build and operate a **Managed Kubernetes** service with a fraction of operational burden. With **Kamaji**, you can deploy and operate hundreds of Kubernetes clusters in the simplest and most automated way.

<p align="center" style="padding: 6px 6px">
  <img src="image.png" />
</p>

## Why we are building it?
One of main driver for Kubernetes success and adoption everywhere, is that Kubernetes represents a standardized interface for deploying Cloud Native software applications. With Kubernetes, software applications can be deployed everywhere from Cloud infrastructures to data-centers and even to Edge and constrained locations.

Kubernetes eliminates de-facto any vendor lockin: once built, a cloud native application can be deployed and operated everywhere Kubernetes is running. This opens a huge opportunity for independent, national and regional providers, even large enterprises, to compete with Public Cloud providers. However, there are not so many tools to build a Managed Kubernetes service out there. Project Kamaji aims to fill this gap.

## Architecture of a Managed Kubernetes service
One of the most structural choices while building a Managed Kubernetes service is how to deploy and operate hundreds of customers’ Kubernetes clusters. All these clusters must be:

* Resilient
* Isolated
* Cost Optimized  

Managing hundreds of Kubernetes cluster would be impossible without the right tools to orchestrate their lifecycle from provisioning to maintaining, upgrating, and finally deleting. So we went with the idea to use Kubernetes itself to orchestrate Kubernetes clusters. This not a really new idea, we just try to make it more efficient :) 

Kamaji creates a Kubernetes cluster we are calling *admin cluster* to deploy other Kubernetes clusters we are calling *tenant clusters*.

As every Kubernetes cluster, the *tenant clusters* have a set of nodes and a control plane, composed of several components: `APIs server`, `scheduler`, `controller manager`. What Kamaji does is to deploy those *tenant clusters* components as pods in the *admin cluster* nodes.

So now we have the stateless components of the *tenant clusters* control plane running as pods in the *admin cluster* nodes. We haven’t mentioned `etcd`, the key-value datastore keeping the state of the cluster, as we will discuss about it later, for the moment let’s only say that it lives outside the *tenant cluster* pods.

And what the customer cluster worker nodes? They are normal Kubernetes nodes: regular instances (virtual or bare metal) connecting to the `API server` of the *tenant cluster* running in one of the *admin cluster* pod.

Our goal is to manage the lifecycle of hundreds of clusters, not only one, so how can we add another *tenant cluster*? As you could expect, Kamaji just deployes a new *tenant cluster* control plane as a new pod in the *admin cluster* and then it joins the *tenant cluster* worker nodes.

```
         ┌──────────────────────────────────────────────────────────┐
         │                                                          │
┌────────►                 multitenant etcd cluster                 ◄─────────┐
│        │                                                          │         │
│        └────────▲───────────────────▲───────────────────▲─────────┘         │
│                 │                   │                   │                   │
│                 │                   │                   │                   │
│                 │                   │                   │                   │
│        ┌────────▼───────────────────▼───────────────────▼─────────┐         │
│        │                                                          │         │
│        │                 admin cluster control plane              │         │
│        │                                                          │         │
│        └────────▲────────────────────▲───────────────────────▲────┘         │
│                 │                    │                       │              │
│                 │                    │                       │              │
│  ┌──────────────▼────────────────────▼───────────────────────▼───────────┐  │
│  │                  tenant control planes running in pods                │  │
│  │ ┌──────────────┐ ┌──────────────┐   ┌──────────────┐ ┌──────────────┐ │  │
│  │ │tenant00      │ │tenant01      │   │tenant02      │ │tenant03      │ │  │
└──┼─►control plane │ │control plane │   │control plane │ │control plane ◄─┼──┘
   │ └▲─────────────┘ └─▲────────────┘   └─▲────────────┘ └──▲───────────┘ │
   └──│─────────────────│──────────────────│─────────────────│─────────────┘
      │                 │                  │                 │
      │  ┌─────────┐    │  ┌─────────┐     │  ┌─────────┐    │  ┌─────────┐
      │  │         │    │  │         │     │  │         │    │  │         │
      │  │ worker  │    │  │ worker  │     │  │ worker  │    │  │ worker  │
      ├──┤         │    ├──┤         │     ├──┤         │    ├──┤         │
      │  └─────────┘    │  └─────────┘     │  └─────────┘    │  └─────────┘
      │                 │                  │                 │
      │  ┌─────────┐    │  ┌─────────┐     │  ┌─────────┐    │  ┌─────────┐
      │  │         │    │  │         │     │  │         │    │  │         │
      ├──┤ worker  │    ├──┤ worker  │     ├──┤ worker  │    ├──┤ worker  │
      │  │         │    │  │         │     │  │         │    │  │         │
      │  └─────────┘    │  └─────────┘     │  └─────────┘    │  └─────────┘
      │                 │                  │                 │
      │  ┌─────────┐    │  ┌─────────┐     │  ┌─────────┐    │  ┌─────────┐
      ├──┤         │    ├──┤         │     ├──┤         │    ├──┤         │
      │  │ worker  │    │  │ worker  │     │  │ worker  │    │  │ worker  │
         │         │       │         │        │         │       │         │
         └─────────┘       └─────────┘        └─────────┘       └─────────┘
```


We have now an architecture that allows us to quickly deploy new *tenant clusters*, but if we go back to our goal, quickly deployment was only part of it, as we want the cluster to be resilient too.

The *admin cluster*, is already resilient, as it's a regular Kubernetes cluster installed with `kubeadm` by Kamaji, so let’s talk about the control plane of the *tenant clusters*, as it’s the specific part of Kamaji architecture.

We are deploying the *tenant cluster* control plane as regular pods in our *admin cluster*. And that means they are as resilient as any other Kubernetes pod. If one of the *tenant cluster* goes down, the `controller manager` of the *admin cluster* will detect it and the pod will be rescheduled and redeployed, without any manual action on our side. To increase the resiliency of *tenant cluster* control plane we can also simply scale up the control plane pods as they are totally stateless. 

## Save the state
Putting the *tenant cluster* control plane in a pod is the easiest part. Also we have to make sure each *tenant cluster* saves the state as to be able to store and retrieve data. All the question is about where and how to deploy `etcd` to make it available to each *tenant cluster*.

Having each *tenant cluster* it own `etcd`, initially it seemed like a good idea. However, he have to take care of `etcd` resiliency and so deploy it with a quorum replicas, i.e three separated instances for each *tenant cluster* with each replicas having its own persistent volume. But managing data persistency in Kubernetes at scale can be cumbersome, leading to the rise of the operational costs in order to mitigate it.

A dedicated `etcd` cluster for each *tenant cluster* doesn’t scale well for a managed service since customers are billed according to the resources of worker nodes they consume, i.e. the control plane is given for free or, at least, applying a small fee. It means that for the service to be competitive it’s important to keep under control the resources consumed by the control planes.

So we have to find an alternative keeping in mind our goal for a resilient and cost optimized solution at same time.

As we can deploy any Kubernetes cluster with an external `etcd` cluster, we want to explore this option for the *tenant cluster* control plane. On the *admin cluster*, we can deploy a multi-tenant `etcd` cluster storing the state of each *tenant cluster*. All the *tenant cluster* control planes would use the same `etcd`, meaning every `APIs server` getting its own space in this multi-tenant `etcd` cluster.

With this solution the resiliency is guaranteed by the usual `etcd` mechanisms, and the pod count remains under control, so it solves the main goal of resiliency and costs optimization. The trade-off here is that we need to operate an external `etcd` cluster, and manage the access control to be sure that every *tenant cluster* access only to its own data.

