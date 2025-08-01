# Terraform

While [Cluster API](https://github.com/kubernetes-sigs/cluster-api) is a common approach for managing Kubernetes infrastructure declaratively, there are situations where Cluster API may not be suitable or desired. This can occur for various reasons, such as:

- The need to keep control plane management separate from infrastructure management
- When the infrastructure provider hosting worker nodes lacks native Cluster API support
- Existing Terraform-based infrastructure workflows that need integration
- Specific compliance or organizational requirements

In these scenarios, an alternative approach is to provision worker nodes using [`yaki`](https://goyaki.clastix.io/), a wrapper around the standard `kubeadm` utility developed and maintained by [Clastix Labs](https://github.com/clastix).

## How It Works

The workflow combines [Terraform](https://developer.hashicorp.com/terraform) for infrastructure provisioning with `yaki` for Kubernetes node bootstrapping:

1. **Terraform** provisions the virtual machines on your chosen infrastructure
2. **`yaki`** installs all required Kubernetes dependencies on each machine
3. **Bootstrap tokens** automatically join the machines to your Kamaji tenant control plane

## Terraform Modules

The [terraform-kamaji-node-pool](https://github.com/clastix/terraform-kamaji-node-pool) repository provides comprehensive Terraform modules for provisioning Kubernetes worker nodes across multiple cloud providers. The repository is structured to support various infrastructure providers with Terraform support, including:

- **AWS** - Auto Scaling Groups with automatic scaling
- **Azure** - Virtual Machine Scale Sets *(planned)*
- **vSphere** - Enterprise-grade virtual machines
- **Proxmox** - Direct VM management on Proxmox VE
- **vCloud** - Multi-tenant VMs on VMware Cloud Director

### Key Features

- **Multi-cloud support** with consistent interfaces across providers
- **Automatic bootstrap token management** for secure cluster joining
- **Shared cloud-init templates** for consistent node configuration
- **Ready-to-use provider implementations** with example configurations
- **Modular architecture** allowing custom integrations

### Getting Started

For detailed usage instructions, see the [project documentation](https://github.com/clastix/terraform-kamaji-node-pool#readme).

!!! tip "Production Considerations"
    The Terraform modules serve as comprehensive examples and starting points for Kamaji integration. While they include production-ready features like security groups, IAM policies, and anti-affinity rules, you should customize them to meet your specific security, compliance, and operational requirements before using them in production environments.

!!! note "Bootstrap Security"
    The modules automatically generate secure bootstrap tokens with limited lifetime and scope. These tokens are used only for the initial node join process and are cleaned up after successful tenent cluster formation.