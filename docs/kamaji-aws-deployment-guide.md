# Setup Kamaji on AWS
This guide will lead you through the process of creating a working Kamaji setup on on Amazon AWS. It requires:

- one bootstrap local workstation
- an EKS Kubernetes cluster to run the Admin and Tenant Control Planes
- an additional `etcd` cluster made of 3 replicas to host the datastore for the Tenants' clusters
- an arbitrary number of AWS virtual machines to host `Tenant`s' workloads

  * [Prepare the bootstrap workspace](#prepare-the-bootstrap-workspace)
  * [Access Admin cluster](#access-admin-cluster)
  * [Setup multi-tenant etcd](#setup-multi-tenant-etcd)
  * [Install Kamaji controller](#install-kamaji-controller)
  * [Create Tenant Cluster](#create-tenant-cluster)
  * [Cleanup](#cleanup)

## Prepare the bootstrap workspace

## Access Admin cluster

## Setup multi-tenant etcd

## Install Kamaji controller

## Create Tenant Cluster

## Cleanup