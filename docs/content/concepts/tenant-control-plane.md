# Tenant Control Plane


Kamaji introduces a new way to manage Kubernetes control planes at scale. Instead of dedicating separate machines to each cluster’s control plane, Kamaji runs every Tenant Cluster’s control plane as a set of pods inside the Management Cluster. This design unlocks significant efficiencies: you can operate hundreds or thousands of isolated Kubernetes clusters on shared infrastructure, all while maintaining strong separation and reliability.

At the heart of this approach is Kamaji’s commitment to upstream compatibility. The control plane components—`kube-apiserver`, `kube-scheduler`, and `kube-controller-manager`—are the same as those used in any CNCF-compliant Kubernetes cluster. Kamaji uses `kubeadm` for setup and lifecycle management, so you get the benefits of a standard, certified Kubernetes experience.

## How It Works

When you want to create a new Tenant Cluster, you simply define a `TenantControlPlane` resource in the Management Cluster. Kamaji’s controllers take over from there, deploying the necessary control plane pods, configuring networking, and connecting to the appropriate datastore. The control plane is exposed via a Kubernetes Service—by default as a `LoadBalancer`, but you can also use `NodePort` or `ClusterIP` depending on your needs.

Worker nodes, whether virtual machines or bare metal, join the Tenant Cluster by connecting to its control plane endpoint. This process is compatible with standard Kubernetes tools and can be automated using Cluster API or other infrastructure automation solutions.

## Highlights

- **Efficiency and Scale:**  
  By running control planes as pods, Kamaji reduces the infrastructure and operational overhead of managing many clusters.

- **High Availability and Automation:**  
  Control plane pods are managed by Kubernetes Deployments, enabling rolling updates, self-healing, and autoscaling. Kamaji automates the entire lifecycle, from creation to deletion.

- **Declarative and GitOps:**  
  The `TenantControlPlane` custom resource allows you to manage clusters declaratively, fitting perfectly with GitOps and Infrastructure as Code workflows.

- **Seamless Integration:**  
  Kamaji works with Cluster API, supports a variety of datastores, and is compatible with the full Kubernetes ecosystem.

Kamaji’s Tenant Control Plane model is designed for organizations that need to deliver robust, production-grade Kubernetes clusters at scale—whether for internal platform engineering, managed services, or multi-tenant environments.

