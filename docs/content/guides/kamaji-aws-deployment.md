# Setup Kamaji on AWS
This guide will lead you through the process of creating a working Kamaji setup on on AWS.

!!! warning ""
    The material here is relatively dense. We strongly encourage you to dedicate time to walk through these instructions, with a mind to learning. We do NOT provide any "one-click" deployment here. However, once you've understood the components involved it is encouraged that you build suitable, auditable GitOps deployment processes around your final infrastructure.

The guide requires:

- a bootstrap machine
- an EKS Kubernetes cluster as the Management cluster 
- an arbitrary number of AWS machines to host `Tenant`s' workloads

