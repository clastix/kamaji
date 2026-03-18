# OpenStack Infra Provider

Use the Cluster API [OpenStack Infra Provider (CAPO)](https://github.com/kubernetes-sigs/cluster-api-provider-openstack) with the Cluster API [Kamaji Control Plane Provider](https://github.com/clastix/cluster-api-control-plane-provider-kamaji) to create Kubernetes clusters.

!!! warning "Important Notes"
    This walkthrough uses an advanced `externalClusterReference` setup. `kind` cluster is used as the Cluster API management cluster for ease of PoC, and it assumes you start with no existing Kubernetes infrastructure, only a working OpenStack environment.

    Because a local `kind` cluster is typically not reachable from OpenStack instances, Kamaji is installed on a kubeadm-based Kubernetes cluster running inside OpenStack (the "control plane cluster"). The tenant control plane is then deployed there using `KamajiControlPlane.spec.deployment.externalClusterReference`.

    If you already have a Kubernetes cluster reachable from your OpenStack nodes (for example, a Magnum-bootstrapped cluster, a kubeadm cluster in OpenStack, or any managed Kubernetes service), you can run both Cluster API controllers and Kamaji on that single management cluster and avoid `externalClusterReference`.

## Topology

```text
                           +--------------------+
                           | management cluster |
                           +---------+----------+
                                    / \
                                   /   \
                                  v     v
          +-----------------------+     +-------------------------+
          | control plane cluster |     | workload tenant cluster |
          +-----------+-----------+     +------------+------------+
                       \                            /
                        +--------------------------+
```

- The management cluster is a local `kind` cluster and runs Cluster API core, Kubeadm providers, CAPO, CAPH (Cluster API Helm add-on provider), and the Kamaji Cluster API control plane provider, and it reconciles both OpenStack `Cluster` resources.
- The control plane cluster is a kubeadm-based cluster on OpenStack that runs Kamaji and add-ons such as CCM, CNI, and CSI.
- The workload tenant cluster runs worker nodes on OpenStack, while its tenant control plane is hosted by Kamaji on the control plane cluster.

## Prerequisites

- `kind`, `kubectl`, `clusterctl`, `openstack` CLI, `openssl`, and `base64`
- Host images built with [image-builder](https://image-builder.sigs.k8s.io/) (or equivalent) and configured with `cloud-init` using the [OpenStack datasource](https://docs.cloud-init.io/en/latest/reference/datasources/openstack.html)
- An existing OpenStack environment with network connectivity, flavors, host images, and sufficient quotas for control plane and worker nodes
- OpenStack load balancer support (Octavia) for Service type `LoadBalancer` and API endpoint/public IP management for the kubeadm-based control plane cluster

!!! warning "ProviderID and cloud-init behavior"
    CAPI host images must have the OpenStack cloud-init datasource configured. Otherwise, kubelet `provider-id` injection can resolve to a Nova-style non-UUID value and Cluster API reconciliation can stall. In this configuration, `kubeletExtraArgs` does not set `provider-id`; OpenStack CCM sets `providerID` after cluster start. See [cloud-init OpenStack datasource docs](https://docs.cloud-init.io/en/latest/reference/datasources/openstack.html) and [CAPO external cloud provider notes](https://cluster-api-openstack.sigs.k8s.io/topics/external-cloud-provider).

## Setup management cluster

```bash
kind create cluster --name management-cluster
kubectl cluster-info --context kind-management-cluster
```

### Install providers

Install ORC before initializing Cluster API providers:

```bash
export ORC_VERSION=v2.0.3
kubectl apply -f "https://github.com/k-orc/openstack-resource-controller/releases/download/${ORC_VERSION}/install.yaml"
```

Initialize Cluster API providers:

```bash
clusterctl init \
  --core cluster-api \
  --bootstrap kubeadm \
  --control-plane kubeadm \
  --infrastructure openstack \
  --addon helm \
  --control-plane kamaji
```

Enable feature gates required by `KamajiControlPlane.spec.deployment.externalClusterReference`:

```bash
kubectl -n kamaji-system patch deployment capi-kamaji-controller-manager \
  --type='json' \
  -p='[
    {
      "op": "replace",
      "path": "/spec/template/spec/containers/0/args/1",
      "value": "--feature-gates=ExternalClusterReference=true,ExternalClusterReferenceCrossNamespace=true"
    }
  ]'
```

### Prepare OpenStack application credentials

Application credentials are used instead of username/password.

!!! warning "Credential Scope"
    The same application credential is reused for both tenant clusters. In production, separate credentials per cluster or environment are preferred.

!!! note "Project selection"
    Application credentials are created in the currently scoped project. Set `TARGET_PROJECT_ID` explicitly so credential creation is project-scoped.

Set and review variables:

```bash
export OPENSTACK_CLOUD_NAME=example-openstack
export TARGET_PROJECT_ID=<REPLACE_WITH_PROJECT_ID>
export OPENSTACK_APP_CREDENTIAL_NAME=capi-kamaji
export OPENSTACK_APP_CREDENTIAL_SECRET="$(openssl rand -hex 24)"
```

Create the application credential:

```bash
openstack --os-cloud "$OPENSTACK_CLOUD_NAME" application credential create \
  --os-project-id "$TARGET_PROJECT_ID" \
  --secret "$OPENSTACK_APP_CREDENTIAL_SECRET" \
  "$OPENSTACK_APP_CREDENTIAL_NAME"
```

Store the generated credential ID:

```bash
export OPENSTACK_APP_CREDENTIAL_ID="$(openstack --os-cloud "$OPENSTACK_CLOUD_NAME" --os-project-id "$TARGET_PROJECT_ID" application credential show "$OPENSTACK_APP_CREDENTIAL_NAME" -f value -c id)"
```

### Prepare `clouds.yaml` and `cloud.conf`

Set OpenStack connection variables:

```bash
export OPENSTACK_AUTH_URL=https://openstack.example.com:5000/v3
export OPENSTACK_REGION_NAME=RegionOne
export OPENSTACK_INTERFACE=public
export OPENSTACK_IDENTITY_API_VERSION=3
export OPENSTACK_TLS_INSECURE=false
```

Build `clouds.yaml` and `cloud.conf` content in environment variables:

```bash
export CLOUDS_YAML_CONTENT="$(cat <<EOF
clouds:
  ${OPENSTACK_CLOUD_NAME}:
    auth:
      auth_url: ${OPENSTACK_AUTH_URL}
      application_credential_id: ${OPENSTACK_APP_CREDENTIAL_ID}
      application_credential_secret: ${OPENSTACK_APP_CREDENTIAL_SECRET}
    auth_type: v3applicationcredential
    region_name: ${OPENSTACK_REGION_NAME}
    interface: ${OPENSTACK_INTERFACE}
    identity_api_version: ${OPENSTACK_IDENTITY_API_VERSION}
EOF
)"

export CLOUD_CONF_CONTENT="$(cat <<EOF
[Global]
auth-url=${OPENSTACK_AUTH_URL}
application-credential-id=${OPENSTACK_APP_CREDENTIAL_ID}
application-credential-secret=${OPENSTACK_APP_CREDENTIAL_SECRET}
region=${OPENSTACK_REGION_NAME}
tls-insecure=${OPENSTACK_TLS_INSECURE}
interface=public
identity-api-version=3
auth-type=v3applicationcredential
EOF
)"
```

Encode both values for Kubernetes Secret `data` fields:

```bash
export REPLACE_BASE64_CLOUD_CONF="$(printf '%s' "$CLOUD_CONF_CONTENT" | base64 | tr -d '\n')"
export REPLACE_BASE64_CLOUDS_YAML="$(printf '%s' "$CLOUDS_YAML_CONTENT" | base64 | tr -d '\n')"
```

## Bootstrap the control plane cluster

### Set variables

Set shared variables used by the next commands:

!!! note "Variable Values"
    Names, versions, network IDs, flavors, and image names in this section are example values. Replace them with environment-specific OpenStack values.

!!! info "Namespace Layout"
    All resources are created in `kamaji-system`. Resources can also be separated by namespace with corresponding RBAC and policy controls.

```bash
export CLUSTER_NAMESPACE=kamaji-system
export CONTROL_PLANE_CLUSTER_NAME=kamaji-control-plane
export WORKLOAD_TENANT_CLUSTER_NAME=tenant-cluster-01
export KUBERNETES_VERSION=v1.33.0
export OPENSTACK_EXTERNAL_NETWORK_ID=00000000-0000-0000-0000-000000000000
export OPENSTACK_FLAVOR=m1.large
export OPENSTACK_IMAGE_NAME=ubuntu-2404-kube-v1.33.0
export OPENSTACK_SSH_KEY_NAME=default
```

### Apply cluster resources

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: ${CONTROL_PLANE_CLUSTER_NAME}
  namespace: ${CLUSTER_NAMESPACE}
  labels:
    clusterctl.cluster.x-k8s.io/move: "true"
type: Opaque
data:
  cacert: ""
  clouds.yaml: ${REPLACE_BASE64_CLOUDS_YAML}
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: ${CONTROL_PLANE_CLUSTER_NAME}
  namespace: ${CLUSTER_NAMESPACE}
  labels:
    addons.cluster.x-k8s.io/ccm: "true"
    addons.cluster.x-k8s.io/cilium: "true"
    addons.cluster.x-k8s.io/cinder-csi: "true"
    addons.cluster.x-k8s.io/cert-manager: "true"
    addons.cluster.x-k8s.io/kamaji: "true"
spec:
  clusterNetwork:
    services:
      cidrBlocks:
      - 10.96.0.0/12
    pods:
      cidrBlocks:
      - 10.244.0.0/16
    serviceDomain: cluster.local
  controlPlaneRef:
    apiVersion: controlplane.cluster.x-k8s.io/v1beta1
    kind: KubeadmControlPlane
    name: ${CONTROL_PLANE_CLUSTER_NAME}-control-plane
    namespace: ${CLUSTER_NAMESPACE}
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: OpenStackCluster
    name: ${CONTROL_PLANE_CLUSTER_NAME}
    namespace: ${CLUSTER_NAMESPACE}
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: OpenStackCluster
metadata:
  name: ${CONTROL_PLANE_CLUSTER_NAME}
  namespace: ${CLUSTER_NAMESPACE}
spec:
  apiServerLoadBalancer:
    enabled: true
  externalNetwork:
    id: ${OPENSTACK_EXTERNAL_NETWORK_ID}
  identityRef:
    cloudName: ${OPENSTACK_CLOUD_NAME}
    name: ${CONTROL_PLANE_CLUSTER_NAME}
  managedSecurityGroups:
    allowAllInClusterTraffic: true
  managedSubnets:
  - cidr: 10.0.0.0/24
    dnsNameservers:
    - 8.8.8.8
---
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
kind: KubeadmControlPlane
metadata:
  name: ${CONTROL_PLANE_CLUSTER_NAME}-control-plane
  namespace: ${CLUSTER_NAMESPACE}
spec:
  replicas: 1
  version: ${KUBERNETES_VERSION}
  machineTemplate:
    infrastructureRef:
      apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
      kind: OpenStackMachineTemplate
      name: ${CONTROL_PLANE_CLUSTER_NAME}-control-plane
      namespace: ${CLUSTER_NAMESPACE}
  kubeadmConfigSpec:
    format: cloud-config
    files:
    - path: /etc/kubernetes/cloud.conf
      owner: root:root
      permissions: "0600"
      encoding: base64
      content: ${REPLACE_BASE64_CLOUD_CONF}
    clusterConfiguration:
      apiServer:
        extraArgs:
          cloud-provider: external
      controllerManager:
        extraArgs:
          cloud-provider: external
    initConfiguration:
      nodeRegistration:
        kubeletExtraArgs:
          cloud-provider: external
        name: '{{ local_hostname }}'
    joinConfiguration:
      nodeRegistration:
        kubeletExtraArgs:
          cloud-provider: external
        name: '{{ local_hostname }}'
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: OpenStackMachineTemplate
metadata:
  name: ${CONTROL_PLANE_CLUSTER_NAME}-control-plane
  namespace: ${CLUSTER_NAMESPACE}
spec:
  template:
    spec:
      flavor: ${OPENSTACK_FLAVOR}
      image:
        filter:
          name: ${OPENSTACK_IMAGE_NAME}
      sshKeyName: ${OPENSTACK_SSH_KEY_NAME}
---
apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
kind: KubeadmConfigTemplate
metadata:
  name: ${CONTROL_PLANE_CLUSTER_NAME}-bootstrap
  namespace: ${CLUSTER_NAMESPACE}
spec:
  template:
    spec:
      format: cloud-config
      files:
      - path: /etc/kubernetes/cloud.conf
        owner: root:root
        permissions: "0600"
        encoding: base64
        content: ${REPLACE_BASE64_CLOUD_CONF}
      joinConfiguration:
        nodeRegistration:
          imagePullPolicy: IfNotPresent
          kubeletExtraArgs:
            cloud-provider: external
          name: '{{ local_hostname }}'
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: OpenStackMachineTemplate
metadata:
  name: ${CONTROL_PLANE_CLUSTER_NAME}-worker
  namespace: ${CLUSTER_NAMESPACE}
spec:
  template:
    spec:
      flavor: ${OPENSTACK_FLAVOR}
      image:
        filter:
          name: ${OPENSTACK_IMAGE_NAME}
      sshKeyName: ${OPENSTACK_SSH_KEY_NAME}
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: ${CONTROL_PLANE_CLUSTER_NAME}-md-0
  namespace: ${CLUSTER_NAMESPACE}
spec:
  clusterName: ${CONTROL_PLANE_CLUSTER_NAME}
  replicas: 1
  selector:
    matchLabels:
      nodepool: worker
  template:
    metadata:
      labels:
        nodepool: worker
    spec:
      clusterName: ${CONTROL_PLANE_CLUSTER_NAME}
      version: ${KUBERNETES_VERSION}
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfigTemplate
          name: ${CONTROL_PLANE_CLUSTER_NAME}-bootstrap
          namespace: ${CLUSTER_NAMESPACE}
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: OpenStackMachineTemplate
        name: ${CONTROL_PLANE_CLUSTER_NAME}-worker
        namespace: ${CLUSTER_NAMESPACE}
EOF
```

### Apply add-on Helm release resources

!!! info "Add-on scheduling profile"
    HelmChartProxy resources are configured to match both the control plane cluster and the workload tenant cluster. The chart values are set to work on both kubeadm-based and Kamaji-based clusters (for example, `nodeSelector: null`, broad tolerations, and `dnsPolicy: Default`).

```bash
kubectl apply -f - <<EOF
apiVersion: addons.cluster.x-k8s.io/v1alpha1
kind: HelmChartProxy
metadata:
  name: openstack-cloud-controller-manager
  namespace: ${CLUSTER_NAMESPACE}
spec:
  clusterSelector:
    matchLabels:
      addons.cluster.x-k8s.io/ccm: "true"
  repoURL: https://kubernetes.github.io/cloud-provider-openstack
  chartName: openstack-cloud-controller-manager
  version: 2.30.1
  namespace: kube-system
  releaseName: openstack-cloud-controller-manager
  options:
    enableClientCache: false
    timeout: 10m0s
    install:
      createNamespace: true
    upgrade:
      maxHistory: 10
  valuesTemplate: |
    secret:
      enabled: false
      create: false

    nodeSelector: null

    tolerations:
      - key: node-role.kubernetes.io/control-plane
        operator: Exists
        effect: NoSchedule
      - key: node.cloudprovider.kubernetes.io/uninitialized
        operator: Equal
        value: "true"
        effect: NoSchedule
      - key: node.kubernetes.io/not-ready
        operator: Exists
        effect: NoSchedule
      - key: node.cluster.x-k8s.io/uninitialized
        operator: Exists
        effect: NoSchedule

    podSecurityContext:
      runAsUser: 0

    dnsPolicy: Default

    extraVolumes:
      - name: cloud-config
        hostPath:
          path: /etc/kubernetes/cloud.conf
          type: File
      - name: k8s-certs
        hostPath:
          path: /etc/kubernetes/pki

    extraVolumeMounts:
      - name: cloud-config
        mountPath: /etc/config/cloud.conf
        readOnly: true
      - name: k8s-certs
        mountPath: /etc/kubernetes/pki
        readOnly: true
---
apiVersion: addons.cluster.x-k8s.io/v1alpha1
kind: HelmChartProxy
metadata:
  name: cilium
  namespace: ${CLUSTER_NAMESPACE}
spec:
  clusterSelector:
    matchLabels:
      addons.cluster.x-k8s.io/cilium: "true"
  repoURL: https://helm.cilium.io
  chartName: cilium
  version: 1.18.4
  namespace: kube-system
  releaseName: cilium
  options:
    enableClientCache: false
    timeout: 10m0s
    install:
      createNamespace: true
    upgrade:
      maxHistory: 10
  valuesTemplate: |
    cni:
      chainingMode: portmap

    prometheus:
      enabled: false

    operator:
      replicas: 1
      prometheus:
        enabled: false

    ipam:
      operator:
        clusterPoolIPv4PodCIDRList:
          - "10.244.0.0/16"
        clusterPoolIPv4MaskSize: 24

    kubeProxyReplacement: false
    sessionAffinity: true

    tolerations:
      - key: node-role.kubernetes.io/control-plane
        operator: Exists
        effect: NoSchedule
      - key: node.cloudprovider.kubernetes.io/uninitialized
        operator: Equal
        value: "true"
        effect: NoSchedule
      - key: node.kubernetes.io/not-ready
        operator: Exists
        effect: NoSchedule
---
apiVersion: addons.cluster.x-k8s.io/v1alpha1
kind: HelmChartProxy
metadata:
  name: openstack-cinder-csi
  namespace: ${CLUSTER_NAMESPACE}
spec:
  clusterSelector:
    matchLabels:
      addons.cluster.x-k8s.io/cinder-csi: "true"
  repoURL: https://kubernetes.github.io/cloud-provider-openstack
  chartName: openstack-cinder-csi
  version: 2.31.7
  namespace: kube-system
  releaseName: openstack-cinder-csi
  options:
    enableClientCache: false
    timeout: 10m0s
    install:
      createNamespace: true
    upgrade:
      maxHistory: 10
  valuesTemplate: |
    csi:
      plugin:
        nodePlugin:
          dnsPolicy: Default
        controllerPlugin:
          dnsPolicy: Default
          nodeSelector: null
          tolerations:
            - key: node-role.kubernetes.io/control-plane
              operator: Exists
              effect: NoSchedule
            - key: node.cloudprovider.kubernetes.io/uninitialized
              operator: Equal
              value: "true"
              effect: NoSchedule
            - key: node.kubernetes.io/not-ready
              operator: Exists
              effect: NoSchedule
            - key: CriticalAddonsOnly
              operator: Exists
            - key: node.cluster.x-k8s.io/uninitialized
              operator: Exists
              effect: NoSchedule

    secret:
      enabled: false
      create: false
      hostMount: true
      filename: cloud.conf

    storageClass:
      enabled: false
      custom: |-
        ---
        apiVersion: storage.k8s.io/v1
        kind: StorageClass
        metadata:
          annotations:
            storageclass.kubernetes.io/is-default-class: "true"
          labels:
            name: cinder-ssd
          name: cinder-ssd
        allowVolumeExpansion: true
        provisioner: cinder.csi.openstack.org
        reclaimPolicy: Delete
        volumeBindingMode: Immediate
        parameters:
          type: ssd
        ---
        apiVersion: storage.k8s.io/v1
        kind: StorageClass
        metadata:
          labels:
            name: cinder-hdd
          name: cinder-hdd
        allowVolumeExpansion: true
        provisioner: cinder.csi.openstack.org
        reclaimPolicy: Delete
        volumeBindingMode: Immediate
        parameters:
          type: hdd
---
apiVersion: addons.cluster.x-k8s.io/v1alpha1
kind: HelmChartProxy
metadata:
  name: cert-manager
  namespace: ${CLUSTER_NAMESPACE}
spec:
  clusterSelector:
    matchLabels:
      addons.cluster.x-k8s.io/cert-manager: "true"
  repoURL: https://charts.jetstack.io
  chartName: cert-manager
  namespace: cert-manager
  releaseName: cert-manager
  options:
    enableClientCache: false
    timeout: 10m0s
    install:
      createNamespace: true
    upgrade:
      maxHistory: 10
  valuesTemplate: |
    installCRDs: true
---
apiVersion: addons.cluster.x-k8s.io/v1alpha1
kind: HelmChartProxy
metadata:
  name: kamaji
  namespace: ${CLUSTER_NAMESPACE}
spec:
  clusterSelector:
    matchLabels:
      addons.cluster.x-k8s.io/kamaji: "true"
  repoURL: https://clastix.github.io/charts
  chartName: kamaji
  version: 0.0.0+latest
  namespace: ${CLUSTER_NAMESPACE}
  releaseName: kamaji
  options:
    enableClientCache: false
    timeout: 10m0s
    install:
      createNamespace: true
    upgrade:
      maxHistory: 10
  valuesTemplate: |
    resources: null
EOF
```

### Verify control plane cluster

Apply and monitor:

```bash
clusterctl describe cluster "$CONTROL_PLANE_CLUSTER_NAME" -n "$CLUSTER_NAMESPACE"
clusterctl get kubeconfig "$CONTROL_PLANE_CLUSTER_NAME" -n "$CLUSTER_NAMESPACE" > ~/.kube/${CONTROL_PLANE_CLUSTER_NAME}.kubeconfig
KUBECONFIG=~/.kube/${CONTROL_PLANE_CLUSTER_NAME}.kubeconfig kubectl get nodes
```

## Bootstrap the tenant cluster with Kamaji control plane

### Set cross-cluster reference

Set the reference to the first cluster kubeconfig Secret:

```bash
export CONTROL_PLANE_CLUSTER_KUBECONFIG_SECRET_NAME="${CONTROL_PLANE_CLUSTER_NAME}-kubeconfig"
```

### Apply tenant cluster resources

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: ${WORKLOAD_TENANT_CLUSTER_NAME}
  namespace: ${CLUSTER_NAMESPACE}
  labels:
    clusterctl.cluster.x-k8s.io/move: "true"
type: Opaque
data:
  cacert: ""
  clouds.yaml: ${REPLACE_BASE64_CLOUDS_YAML}
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: ${WORKLOAD_TENANT_CLUSTER_NAME}
  namespace: ${CLUSTER_NAMESPACE}
  labels:
    addons.cluster.x-k8s.io/ccm: "true"
    addons.cluster.x-k8s.io/cilium: "true"
    addons.cluster.x-k8s.io/cinder-csi: "true"
spec:
  clusterNetwork:
    services:
      cidrBlocks:
      - 10.96.0.0/12
    pods:
      cidrBlocks:
      - 10.244.0.0/16
    serviceDomain: cluster.local
  controlPlaneRef:
    apiVersion: controlplane.cluster.x-k8s.io/v1alpha1
    kind: KamajiControlPlane
    name: ${WORKLOAD_TENANT_CLUSTER_NAME}
    namespace: ${CLUSTER_NAMESPACE}
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: OpenStackCluster
    name: ${WORKLOAD_TENANT_CLUSTER_NAME}
    namespace: ${CLUSTER_NAMESPACE}
---
apiVersion: controlplane.cluster.x-k8s.io/v1alpha1
kind: KamajiControlPlane
metadata:
  name: ${WORKLOAD_TENANT_CLUSTER_NAME}
  namespace: ${CLUSTER_NAMESPACE}
spec:
  version: ${KUBERNETES_VERSION}
  dataStoreName: default
  replicas: 1
  apiServer:
    extraArgs:
    - --cloud-provider=external
  controllerManager:
    extraArgs:
    - --cloud-provider=external
  kubelet:
    cgroupfs: systemd
    preferredAddressTypes:
    - InternalIP
    configurationJSONPatches:
    - op: remove
      path: /imagePullCredentialsVerificationPolicy
    - op: remove
      path: /mergeDefaultEvictionSettings
    - op: remove
      path: /crashLoopBackOff
    - op: add
      path: /cgroupDriver
      value: systemd
  network:
    serviceType: LoadBalancer
  addons:
    coreDNS: {}
    kubeProxy: {}
    konnectivity: {}
  deployment:
    externalClusterReference:
      deploymentNamespace: ${CLUSTER_NAMESPACE}
      kubeconfigSecretName: ${CONTROL_PLANE_CLUSTER_KUBECONFIG_SECRET_NAME}
      kubeconfigSecretKey: value
      kubeconfigSecretNamespace: ${CLUSTER_NAMESPACE}
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: OpenStackCluster
metadata:
  name: ${WORKLOAD_TENANT_CLUSTER_NAME}
  namespace: ${CLUSTER_NAMESPACE}
spec:
  apiServerLoadBalancer:
    enabled: false
  disableAPIServerFloatingIP: true
  externalNetwork:
    id: ${OPENSTACK_EXTERNAL_NETWORK_ID}
  identityRef:
    cloudName: ${OPENSTACK_CLOUD_NAME}
    name: ${WORKLOAD_TENANT_CLUSTER_NAME}
  managedSecurityGroups:
    allowAllInClusterTraffic: true
  managedSubnets:
  - cidr: 10.0.0.0/24
    dnsNameservers:
    - 8.8.8.8
---
apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
kind: KubeadmConfigTemplate
metadata:
  name: ${WORKLOAD_TENANT_CLUSTER_NAME}-bootstrap
  namespace: ${CLUSTER_NAMESPACE}
spec:
  template:
    spec:
      format: cloud-config
      files:
      - path: /etc/kubernetes/cloud.conf
        owner: root:root
        permissions: "0600"
        encoding: base64
        content: ${REPLACE_BASE64_CLOUD_CONF}
      joinConfiguration:
        nodeRegistration:
          imagePullPolicy: IfNotPresent
          kubeletExtraArgs:
            cloud-provider: external
          name: '{{ local_hostname }}'
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: OpenStackMachineTemplate
metadata:
  name: ${WORKLOAD_TENANT_CLUSTER_NAME}-worker
  namespace: ${CLUSTER_NAMESPACE}
spec:
  template:
    spec:
      flavor: ${OPENSTACK_FLAVOR}
      image:
        filter:
          name: ${OPENSTACK_IMAGE_NAME}
      sshKeyName: ${OPENSTACK_SSH_KEY_NAME}
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: ${WORKLOAD_TENANT_CLUSTER_NAME}-md-0
  namespace: ${CLUSTER_NAMESPACE}
spec:
  clusterName: ${WORKLOAD_TENANT_CLUSTER_NAME}
  replicas: 1
  selector:
    matchLabels:
      nodepool: worker
  template:
    metadata:
      labels:
        nodepool: worker
    spec:
      clusterName: ${WORKLOAD_TENANT_CLUSTER_NAME}
      version: ${KUBERNETES_VERSION}
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfigTemplate
          name: ${WORKLOAD_TENANT_CLUSTER_NAME}-bootstrap
          namespace: ${CLUSTER_NAMESPACE}
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: OpenStackMachineTemplate
        name: ${WORKLOAD_TENANT_CLUSTER_NAME}-worker
        namespace: ${CLUSTER_NAMESPACE}
EOF
```

### Verify tenant cluster

Apply and monitor:

```bash
clusterctl describe cluster "$WORKLOAD_TENANT_CLUSTER_NAME" -n "$CLUSTER_NAMESPACE"
clusterctl get kubeconfig "$WORKLOAD_TENANT_CLUSTER_NAME" -n "$CLUSTER_NAMESPACE" > ~/.kube/${WORKLOAD_TENANT_CLUSTER_NAME}.kubeconfig
KUBECONFIG=~/.kube/${WORKLOAD_TENANT_CLUSTER_NAME}.kubeconfig kubectl get nodes
```

## Clean up

```bash
kubectl delete cluster "$WORKLOAD_TENANT_CLUSTER_NAME" -n "$CLUSTER_NAMESPACE"
kubectl delete cluster "$CONTROL_PLANE_CLUSTER_NAME" -n "$CLUSTER_NAMESPACE"
kind delete cluster --name management-cluster
```
