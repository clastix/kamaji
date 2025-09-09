# Cluster Class with Kamaji

`ClusterClass` is a Cluster API experimental feature that enables template-based cluster creation. When combined with Kamaji's hosted control plane architecture, `ClusterClass` provides a powerful pattern for standardizing Kubernetes cluster deployments across multiple infrastructure providers while maintaining consistent control plane configurations.

!!! warning "Experimental Feature"
    `ClusterClass` is an experimental feature of Cluster API. It requires Kubernetes >= 1.22.0 and explicit feature gate enablement. As with any experimental features, use with caution in production environments.

## Understanding Cluster Class

`ClusterClass` reduces configuration boilerplate by defining reusable cluster templates. Instead of creating individual resources for each cluster, you define a `ClusterClass` once and create multiple clusters from it with minimal configuration.

With Kamaji, this pattern becomes even more powerful:
- **Shared Control Plane Templates**: The same KamajiControlPlaneTemplate works across all infrastructure providers
- **Infrastructure Flexibility**: Deploy worker nodes on vSphere, AWS, Azure, or any supported provider while maintaining consistent control planes
- **Simplified Management**: Hosted control planes reduce the complexity of `ClusterClass` templates

## Enabling Cluster Class

To use `ClusterClass` with Kamaji, you need to enable the cluster topology feature gate before initializing the management cluster:

```bash
export CLUSTER_TOPOLOGY=true
clusterctl init --control-plane kamaji --infrastructure vsphere
```

This will install:
- Cluster API core components with `ClusterClass` support
- Kamaji Control Plane Provider
- Your chosen infrastructure provider (vSphere in this example)

Verify the installation:

```bash
kubectl get deployments -A | grep -E "capi|kamaji"
```

## Template Architecture with Kamaji

A `ClusterClass` with Kamaji consists of four main components:

1. Control Plane Template (KamajiControlPlaneTemplate): Defines the hosted control plane configuration that remains consistent across infrastructure providers.

2. Infrastructure Template (VSphereClusterTemplate): Provider-specific infrastructure configuration for the cluster.

3. Bootstrap Template (KubeadmConfigTemplate): Node initialization configuration that works across providers.

4. Machine Template (VSphereMachineTemplate): Provider-specific machine configuration for worker nodes.

Here's how these components relate in a `ClusterClass`:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: ClusterClass
metadata:
  name: kamaji-vsphere-class
spec:
  # Infrastructure provider template
  infrastructure:
    ref:
      apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
      kind: VSphereClusterTemplate
      name: vsphere-cluster-template
  
  # Kamaji control plane template - reusable across providers
  controlPlane:
    ref:
      apiVersion: controlplane.cluster.x-k8s.io/v1alpha1
      kind: KamajiControlPlaneTemplate
      name: kamaji-control-plane-template
    
  # Worker configuration
  workers:
    machineDeployments:
    - class: default-worker
      template:
        bootstrap:
          ref:
            apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
            kind: KubeadmConfigTemplate
            name: worker-bootstrap-template
        infrastructure:
          ref:
            apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
            kind: VSphereMachineTemplate
            name: vsphere-worker-template
```

The key advantage: the KamajiControlPlaneTemplate and KubeadmConfigTemplate can be shared across different infrastructure providers, while only the infrastructure-specific templates need to change.

## Creating a Cluster Class

Let's create a `ClusterClass` for vSphere with Kamaji. First, define the shared templates:

### KamajiControlPlaneTemplate

This template defines the hosted control plane configuration:

```yaml
apiVersion: controlplane.cluster.x-k8s.io/v1alpha1
kind: KamajiControlPlaneTemplate
metadata:
  name: kamaji-controlplane
  namespace: capi-templates-vsphere
spec:
  template:
    spec:
      dataStoreName: "default"  # Default datastore for etcd
      
      network:
        serviceType: LoadBalancer
        serviceAddress: ""
        certSANs: []
        
      addons:
        coreDNS: {}
        kubeProxy: {}
        konnectivity: {}

      apiServer:
        extraArgs: []
        resources:
          requests: {}
      controllerManager:
        extraArgs: []
        resources:
          requests: {}
      scheduler:
        extraArgs: []
        resources:
          requests: {}
            
      kubelet:
        cgroupfs: systemd
        preferredAddressTypes:
        - InternalIP
        
      registry: "registry.k8s.io"
```

### KubeadmConfigTemplate

This bootstrap template configures worker nodes:

```yaml
apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
kind: KubeadmConfigTemplate
metadata:
  name: worker-bootstrap-template
spec:
  template:
    spec:
    
      # Configuration for kubeadm join
      joinConfiguration:
        discovery: {}
        nodeRegistration:
          criSocket: /var/run/containerd/containerd.sock
          imagePullPolicy: IfNotPresent
          name: '{{ local_hostname }}'
          kubeletExtraArgs:
            cloud-provider: external
            node-ip: "{{ ds.meta_data.local_ipv4 }}"       

      # Commands to run before kubeadm join
      preKubeadmCommands:
      - hostnamectl set-hostname "{{ ds.meta_data.hostname }}"
      - echo "127.0.0.1 {{ ds.meta_data.hostname }}" >> /etc/hosts
      
      # Commands to run after kubeadm join
      postKubeadmCommands: []

      # Users to create on worker nodes
      users: []
```

### VSphereClusterTemplate

Infrastructure-specific template for vSphere:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: VSphereClusterTemplate
metadata:
  name: vsphere
  namespace: capi-templates-vsphere
spec:
  template:
    spec:
      server: "vcenter.sample.com"  # vCenter server address
      thumbprint: ""                # vCenter certificate thumbprint
      
      identityRef:
        kind: VSphereClusterIdentity  
        name: "vsphere-cluster-identity"
      
      failureDomainSelector: {} 
      clusterModules: []
```

### VSphereMachineTemplate

Machine template for vSphere workers:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: VSphereMachineTemplate
metadata:
  name: vsphere-vm-base
  namespace: capi-templates-vsphere
spec:
  template:
    spec:
      # Resources will be patched by ClusterClass based on variables
      # numCPUs, memoryMiB, diskGiB are dynamically set
      
      # Infrastructure defaults - will be patched by ClusterClass
      server: "vcenter.sample.com"
      datacenter: "datacenter"
      datastore: "datastore"
      resourcePool: "Resources"
      folder: "vm-folder"
      template: "ubuntu-2404-kube-v1.32.0"
      storagePolicyName: ""
      thumbprint: ""
      
      # Network configuration (IPAM by default)
      network:
        devices:
        - networkName: "k8s-network"
          dhcp4: false
          addressesFromPools:
          - apiGroup: ipam.cluster.x-k8s.io
            kind: InClusterIPPool
            name: "{{ .builtin.cluster.name }}"  # Uses cluster name
```

### Variables and Patching in Cluster Class

`ClusterClass` becomes powerful through its variable system and JSON patching capabilities. This allows the same templates to be customized for different use cases without duplicating YAML.

#### Variable System

Variables in `ClusterClass` define the parameters users can customize when creating clusters. Each variable has:

- **Schema Definition**: OpenAPI v3 schema that validates input
- **Required/Optional**: Whether the variable must be provided
- **Default Values**: Fallback values when not specified
- **Type Constraints**: Data types, ranges, and enum values

Here's how variables work in practice:

**Control Plane Variables:**
```yaml
variables:
- name: kamajiControlPlane
  required: true
  schema:
    openAPIV3Schema:
      type: object
      properties:
        dataStoreName:
          type: string
          description: "Datastore name for etcd"
          default: "default"
        network:
          type: object
          properties:
            serviceType:
              type: string
              enum: ["ClusterIP", "NodePort", "LoadBalancer"]
              default: "LoadBalancer"
            serviceAddress:
              type: string
              description: "Pre-assigned VIP address"
```

**Machine Resource Variables:**
```yaml
- name: machineSpecs
  required: true
  schema:
    openAPIV3Schema:
      type: object
      properties:
        numCPUs:
          type: integer
          minimum: 2
          maximum: 64
          default: 4
        memoryMiB:
          type: integer
          minimum: 4096
          maximum: 131072
          default: 8192
        diskGiB:
          type: integer
          minimum: 40
          maximum: 2048
          default: 100
```

#### JSON Patching System

Patches apply variable values to the base templates at cluster creation time. This enables the same template to serve different configurations.

**Control Plane Patching:**
```yaml
patches:
- name: controlPlaneConfig
  definitions:
  - selector:
      apiVersion: controlplane.cluster.x-k8s.io/v1alpha1
      kind: KamajiControlPlaneTemplate
      matchResources:
        controlPlane: true
    jsonPatches:
    - op: replace
      path: /spec/template/spec/dataStoreName
      valueFrom:
        variable: kamajiControlPlane.dataStoreName
    - op: replace
      path: /spec/template/spec/network/serviceType
      valueFrom:
        variable: kamajiControlPlane.network.serviceType
```

**Machine Resource Patching:**
```yaml
- name: machineResources
  definitions:
  - selector:
      apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
      kind: VSphereMachineTemplate
      matchResources:
        machineDeploymentClass:
          names: ["default-worker"]
    jsonPatches:
    - op: add  # Resources are not in base template
      path: /spec/template/spec/numCPUs
      valueFrom:
        variable: machineSpecs.numCPUs
    - op: add
      path: /spec/template/spec/memoryMiB
      valueFrom:
        variable: machineSpecs.memoryMiB
```

#### Advanced Patching Patterns

**Conditional Patching:**
```yaml
- name: optionalVIP
  definitions:
  - selector:
      apiVersion: controlplane.cluster.x-k8s.io/v1alpha1
      kind: KamajiControlPlaneTemplate
    jsonPatches:
    - op: replace
      path: /spec/template/spec/network/serviceAddress
      valueFrom:
        variable: kamajiControlPlane.network.serviceAddress
      # Only applies if serviceAddress is not empty
      enabledIf: "{{ ne .kamajiControlPlane.network.serviceAddress \"\" }}"
```

**Infrastructure Patching:**
```yaml
- name: infrastructureConfig
  definitions:
  - selector:
      apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
      kind: VSphereMachineTemplate
    jsonPatches:
    - op: replace
      path: /spec/template/spec/datacenter
      valueFrom:
        variable: infrastructure.datacenter
    - op: replace
      path: /spec/template/spec/datastore
      valueFrom:
        variable: infrastructure.datastore
    - op: replace
      path: /spec/template/spec/template
      valueFrom:
        variable: infrastructure.vmTemplate
```

### Complete Cluster Class with Variables

The complete `ClusterClass` combines all templates with comprehensive variable definitions and patches. This creates a flexible foundation for cluster provisioning while maintaining consistency.

For a comprehensive example with all variables and patches configured, see the [vsphere-kamaji-clusterclass.yaml](https://raw.githubusercontent.com/clastix/cluster-api-control-plane-provider-kamaji/master/templates/vsphere/capi-kamaji-vsphere-class-template.yaml) template.

Apply the `ClusterClass` and its templates:

```bash
kubectl apply -f vsphere-standard-clusterclass.yaml
```

Verify the `ClusterClass` is ready:

```bash
kubectl get clusterclass vsphere-standard -n capi-templates-vsphere
```

## Creating a Cluster from Cluster Class

With the `ClusterClass` defined, creating a cluster becomes remarkably simple:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: my-cluster
  namespace: default
spec:
  # Network configuration defined at cluster level
  clusterNetwork:
    pods:
      cidrBlocks: ["10.244.0.0/16"]
    services:
      cidrBlocks: ["10.96.0.0/12"]
    serviceDomain: "cluster.local"
  
  topology:
    class: vsphere-standard
    classNamespace: capi-templates-vsphere
    version: v1.32.0
    
    controlPlane:
      replicas: 2
    
    workers:
      machineDeployments:
      - class: default-worker
        name: worker-nodes
        replicas: 3
    
    variables:
    - name: kamajiControlPlane
      value:
        dataStoreName: "etcd"
        network:
          serviceType: "LoadBalancer"
          serviceAddress: ""  # Auto-assigned if empty
    
    - name: machineSpecs
      value:
        numCPUs: 8
        memoryMiB: 16384
        diskGiB: 60
    
    - name: infrastructure
      value:
        vmTemplate: "ubuntu-2404-kube-v1.32.0"
        datacenter: "K8s-TI-dtc"
        datastore: "K8s-N01td-01"
        resourcePool: "rp-kamaji-dev"
        folder: "my-cluster-vms"
    
    - name: networking
      value:
        networkName: "VM-K8s-TI-cpmgmt"
        nameservers: ["8.8.8.8", "1.1.1.1"]
        dhcp4: false  # Using IPAM
```

Create the cluster:

```bash
kubectl apply -f my-cluster.yaml
```

Monitor cluster creation:

```bash
clusterctl describe cluster my-cluster
kubectl get cluster,kamajicontrolplane,machinedeployment -n default
```

With this approach, the same `KamajiControlPlaneTemplate` and `KubeadmConfigTemplate` can be reused when creating `ClusterClasses` for AWS, Azure, or any other provider. Only the infrastructure-specific templates need to change.

## Cross-Provider Template Reuse

One of Kamaji's key advantages with `ClusterClass` is template modularity across providers. Here's how to leverage this:

### Shared Templates Repository

Create a namespace for shared templates:

```bash
kubectl create namespace cluster-templates
```

Deploy shared Kamaji and bootstrap templates once:

```bash
kubectl apply -n cluster-templates -f kamaji-controlplane-template.yaml
kubectl apply -n cluster-templates -f kubeadm-config-template.yaml
```

### Provider-Specific Cluster Classes

For each infrastructure provider, create a `ClusterClass` that references the shared templates:

#### AWS Cluster Class

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: ClusterClass
metadata:
  name: kamaji-aws-class
spec:
  controlPlane:
    ref:
      apiVersion: controlplane.cluster.x-k8s.io/v1alpha1
      kind: KamajiControlPlaneTemplate
      name: kamaji-controlplane
      namespace: cluster-templates  # Shared template
  
  infrastructure:
    ref:
      apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
      kind: AWSClusterTemplate
      name: aws-cluster-template  # AWS-specific
  
  workers:
    machineDeployments:
    - class: default-worker
      template:
        bootstrap:
          ref:
            apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
            kind: KubeadmConfigTemplate
            name: kubeadm
            namespace: cluster-templates  # Shared template
        infrastructure:
          ref:
            apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
            kind: AWSMachineTemplate
            name: aws-worker-template  # AWS-specific
```

### Azure Cluster Class

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: ClusterClass
metadata:
  name: kamaji-azure-class
spec:
  controlPlane:
    ref:
      apiVersion: controlplane.cluster.x-k8s.io/v1alpha1
      kind: KamajiControlPlaneTemplate
      name: kamaji-control-plane-template
      namespace: cluster-templates  # Same shared template
  
  infrastructure:
    ref:
      apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
      kind: AzureClusterTemplate
      name: azure-cluster-template  # Azure-specific
  
  workers:
    machineDeployments:
    - class: default-worker
      template:
        bootstrap:
          ref:
            apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
            kind: KubeadmConfigTemplate
            name: worker-bootstrap-template
            namespace: cluster-templates  # Same shared template
        infrastructure:
          ref:
            apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
            kind: AzureMachineTemplate
            name: azure-worker-template  # Azure-specific
```

## Managing Cluster Class Lifecycle

### Listing Available Cluster Classes

```bash
kubectl get clusterclasses -A
```

### Viewing Cluster Class Details

```bash
kubectl describe clusterclass vsphere-standard -n capi-templates-vsphere
```

### Updating a Cluster Class

A `ClusterClass` update affects only new clusters. Existing clusters continue using their original configuration:

```bash
kubectl edit clusterclass vsphere-standard -n capi-templates-vsphere
```

### Deleting Clusters Created from Cluster Class

Always delete clusters before removing the `ClusterClass`:

```bash
# Delete the cluster
kubectl delete cluster my-cluster

# Wait for cleanup
kubectl wait --for=delete cluster/my-cluster --timeout=10m

# Then safe to delete ClusterClass if no longer needed
kubectl delete clusterclass vsphere-standard -n capi-templates-vsphere
```

## Template Versioning Strategies

When managing `ClusterClasses` across environments, consider these versioning approaches:

### Semantic Versioning in Names

```yaml
metadata:
  name: vsphere-standard-v1-2-0
  namespace: capi-templates-vsphere
```

### Using Labels for Version Tracking

```yaml
metadata:
  name: vsphere-standard
  namespace: capi-templates-vsphere
  labels:
    version: "1.2.0"
    stability: "stable"
    tier: "standard"
```

### Namespace Separation

```bash
kubectl create namespace clusterclass-v1
kubectl create namespace clusterclass-v2
```

This enables gradual migration between `ClusterClass` versions while maintaining compatibility.

## Further Reading

  - [Cluster API ClusterClass Documentation](https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/)
  - [Kamaji Control Plane Provider Reference](https://doc.crds.dev/github.com/clastix/cluster-api-control-plane-provider-kamaji)
  - [CAPI Provider Integration](https://github.com/clastix/cluster-api-control-plane-provider-kamaji)