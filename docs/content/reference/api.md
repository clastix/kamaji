# API Reference

Packages:

- [kamaji.clastix.io/v1alpha1](#kamajiclastixiov1alpha1)

# kamaji.clastix.io/v1alpha1

Resource Types:

- [DataStore](#datastore)

- [TenantControlPlane](#tenantcontrolplane)




## DataStore






DataStore is the Schema for the datastores API.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>kamaji.clastix.io/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>DataStore</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#datastorespec">spec</a></b></td>
        <td>object</td>
        <td>
          DataStoreSpec defines the desired state of DataStore.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#datastorestatus">status</a></b></td>
        <td>object</td>
        <td>
          DataStoreStatus defines the observed state of DataStore.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DataStore.spec



DataStoreSpec defines the desired state of DataStore.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>driver</b></td>
        <td>enum</td>
        <td>
          The driver to use to connect to the shared datastore.<br/>
          <br/>
            <i>Enum</i>: etcd, MySQL, PostgreSQL<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>endpoints</b></td>
        <td>[]string</td>
        <td>
          List of the endpoints to connect to the shared datastore. No need for protocol, just bare IP/FQDN and port.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#datastorespectlsconfig">tlsConfig</a></b></td>
        <td>object</td>
        <td>
          Defines the TLS/SSL configuration required to connect to the data store in a secure way.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#datastorespecbasicauth">basicAuth</a></b></td>
        <td>object</td>
        <td>
          In case of authentication enabled for the given data store, specifies the username and password pair. This value is optional.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DataStore.spec.tlsConfig



Defines the TLS/SSL configuration required to connect to the data store in a secure way.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#datastorespectlsconfigcertificateauthority">certificateAuthority</a></b></td>
        <td>object</td>
        <td>
          Retrieve the Certificate Authority certificate and private key, such as bare content of the file, or a SecretReference. The key reference is required since etcd authentication is based on certificates, and Kamaji is responsible in creating this.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#datastorespectlsconfigclientcertificate">clientCertificate</a></b></td>
        <td>object</td>
        <td>
          Specifies the SSL/TLS key and private key pair used to connect to the data store.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### DataStore.spec.tlsConfig.certificateAuthority



Retrieve the Certificate Authority certificate and private key, such as bare content of the file, or a SecretReference. The key reference is required since etcd authentication is based on certificates, and Kamaji is responsible in creating this.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#datastorespectlsconfigcertificateauthoritycertificate">certificate</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#datastorespectlsconfigcertificateauthorityprivatekey">privateKey</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DataStore.spec.tlsConfig.certificateAuthority.certificate





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>content</b></td>
        <td>string</td>
        <td>
          Bare content of the file, base64 encoded. It has precedence over the SecretReference value.<br/>
          <br/>
            <i>Format</i>: byte<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#datastorespectlsconfigcertificateauthoritycertificatesecretreference">secretReference</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DataStore.spec.tlsConfig.certificateAuthority.certificate.secretReference





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>keyPath</b></td>
        <td>string</td>
        <td>
          Name of the key for the given Secret reference where the content is stored. This value is mandatory.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is unique within a namespace to reference a secret resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace defines the space within which the secret name must be unique.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DataStore.spec.tlsConfig.certificateAuthority.privateKey





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>content</b></td>
        <td>string</td>
        <td>
          Bare content of the file, base64 encoded. It has precedence over the SecretReference value.<br/>
          <br/>
            <i>Format</i>: byte<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#datastorespectlsconfigcertificateauthorityprivatekeysecretreference">secretReference</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DataStore.spec.tlsConfig.certificateAuthority.privateKey.secretReference





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>keyPath</b></td>
        <td>string</td>
        <td>
          Name of the key for the given Secret reference where the content is stored. This value is mandatory.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is unique within a namespace to reference a secret resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace defines the space within which the secret name must be unique.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DataStore.spec.tlsConfig.clientCertificate



Specifies the SSL/TLS key and private key pair used to connect to the data store.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#datastorespectlsconfigclientcertificatecertificate">certificate</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#datastorespectlsconfigclientcertificateprivatekey">privateKey</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### DataStore.spec.tlsConfig.clientCertificate.certificate





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>content</b></td>
        <td>string</td>
        <td>
          Bare content of the file, base64 encoded. It has precedence over the SecretReference value.<br/>
          <br/>
            <i>Format</i>: byte<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#datastorespectlsconfigclientcertificatecertificatesecretreference">secretReference</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DataStore.spec.tlsConfig.clientCertificate.certificate.secretReference





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>keyPath</b></td>
        <td>string</td>
        <td>
          Name of the key for the given Secret reference where the content is stored. This value is mandatory.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is unique within a namespace to reference a secret resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace defines the space within which the secret name must be unique.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DataStore.spec.tlsConfig.clientCertificate.privateKey





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>content</b></td>
        <td>string</td>
        <td>
          Bare content of the file, base64 encoded. It has precedence over the SecretReference value.<br/>
          <br/>
            <i>Format</i>: byte<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#datastorespectlsconfigclientcertificateprivatekeysecretreference">secretReference</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DataStore.spec.tlsConfig.clientCertificate.privateKey.secretReference





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>keyPath</b></td>
        <td>string</td>
        <td>
          Name of the key for the given Secret reference where the content is stored. This value is mandatory.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is unique within a namespace to reference a secret resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace defines the space within which the secret name must be unique.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DataStore.spec.basicAuth



In case of authentication enabled for the given data store, specifies the username and password pair. This value is optional.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#datastorespecbasicauthpassword">password</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#datastorespecbasicauthusername">username</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### DataStore.spec.basicAuth.password





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>content</b></td>
        <td>string</td>
        <td>
          Bare content of the file, base64 encoded. It has precedence over the SecretReference value.<br/>
          <br/>
            <i>Format</i>: byte<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#datastorespecbasicauthpasswordsecretreference">secretReference</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DataStore.spec.basicAuth.password.secretReference





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>keyPath</b></td>
        <td>string</td>
        <td>
          Name of the key for the given Secret reference where the content is stored. This value is mandatory.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is unique within a namespace to reference a secret resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace defines the space within which the secret name must be unique.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DataStore.spec.basicAuth.username





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>content</b></td>
        <td>string</td>
        <td>
          Bare content of the file, base64 encoded. It has precedence over the SecretReference value.<br/>
          <br/>
            <i>Format</i>: byte<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#datastorespecbasicauthusernamesecretreference">secretReference</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DataStore.spec.basicAuth.username.secretReference





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>keyPath</b></td>
        <td>string</td>
        <td>
          Name of the key for the given Secret reference where the content is stored. This value is mandatory.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is unique within a namespace to reference a secret resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace defines the space within which the secret name must be unique.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DataStore.status



DataStoreStatus defines the observed state of DataStore.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>usedBy</b></td>
        <td>[]string</td>
        <td>
          List of the Tenant Control Planes, namespaced named, using this data store.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

## TenantControlPlane






TenantControlPlane is the Schema for the tenantcontrolplanes API.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>kamaji.clastix.io/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>TenantControlPlane</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespec">spec</a></b></td>
        <td>object</td>
        <td>
          TenantControlPlaneSpec defines the desired state of TenantControlPlane.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatus">status</a></b></td>
        <td>object</td>
        <td>
          TenantControlPlaneStatus defines the observed state of TenantControlPlane.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec



TenantControlPlaneSpec defines the desired state of TenantControlPlane.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplane">controlPlane</a></b></td>
        <td>object</td>
        <td>
          ControlPlane defines how the Tenant Control Plane Kubernetes resources must be created in the Admin Cluster, such as the number of Pod replicas, the Service resource, or the Ingress.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeckubernetes">kubernetes</a></b></td>
        <td>object</td>
        <td>
          Kubernetes specification for tenant control plane<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespecaddons">addons</a></b></td>
        <td>object</td>
        <td>
          Addons contain which addons are enabled<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>dataStore</b></td>
        <td>string</td>
        <td>
          DataStore allows to specify a DataStore that should be used to store the Kubernetes data for the given Tenant Control Plane. This parameter is optional and acts as an override over the default one which is used by the Kamaji Operator. Migration from a different DataStore to another one is not yet supported and the reconciliation will be blocked.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespecnetworkprofile">networkProfile</a></b></td>
        <td>object</td>
        <td>
          NetworkProfile specifies how the network is<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane



ControlPlane defines how the Tenant Control Plane Kubernetes resources must be created in the Admin Cluster, such as the number of Pod replicas, the Service resource, or the Ingress.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplaneservice">service</a></b></td>
        <td>object</td>
        <td>
          Defining the options for the Tenant Control Plane Service resource.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeployment">deployment</a></b></td>
        <td>object</td>
        <td>
          Defining the options for the deployed Tenant Control Plane as Deployment resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplaneingress">ingress</a></b></td>
        <td>object</td>
        <td>
          Defining the options for an Optional Ingress which will expose API Server of the Tenant Control Plane<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.service



Defining the options for the Tenant Control Plane Service resource.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>serviceType</b></td>
        <td>enum</td>
        <td>
          ServiceType allows specifying how to expose the Tenant Control Plane.<br/>
          <br/>
            <i>Enum</i>: ClusterIP, NodePort, LoadBalancer<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplaneserviceadditionalmetadata">additionalMetadata</a></b></td>
        <td>object</td>
        <td>
          AdditionalMetadata defines which additional metadata, such as labels and annotations, must be attached to the created resource.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.service.additionalMetadata



AdditionalMetadata defines which additional metadata, such as labels and annotations, must be attached to the created resource.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>annotations</b></td>
        <td>map[string]string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>labels</b></td>
        <td>map[string]string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment



Defining the options for the deployed Tenant Control Plane as Deployment resource.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentadditionalmetadata">additionalMetadata</a></b></td>
        <td>object</td>
        <td>
          AdditionalMetadata defines which additional metadata, such as labels and annotations, must be attached to the created resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinity">affinity</a></b></td>
        <td>object</td>
        <td>
          If specified, the Tenant Control Plane pod's scheduling constraints. More info: https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentextraargs">extraArgs</a></b></td>
        <td>object</td>
        <td>
          ExtraArgs allows adding additional arguments to the Control Plane components, such as kube-apiserver, controller-manager, and scheduler.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>nodeSelector</b></td>
        <td>map[string]string</td>
        <td>
          NodeSelector is a selector which must be true for the pod to fit on a node. Selector which must match a node's labels for the pod to be scheduled on that node. More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>replicas</b></td>
        <td>integer</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Default</i>: 2<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentresources">resources</a></b></td>
        <td>object</td>
        <td>
          Resources defines the amount of memory and CPU to allocate to each component of the Control Plane (kube-apiserver, controller-manager, and scheduler).<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>runtimeClassName</b></td>
        <td>string</td>
        <td>
          RuntimeClassName refers to a RuntimeClass object in the node.k8s.io group, which should be used to run the Tenant Control Plane pod. If no RuntimeClass resource matches the named class, the pod will not be run. If unset or empty, the "legacy" RuntimeClass will be used, which is an implicit class with an empty definition that uses the default runtime handler. More info: https://git.k8s.io/enhancements/keps/sig-node/585-runtime-class<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentstrategy">strategy</a></b></td>
        <td>object</td>
        <td>
          Strategy describes how to replace existing pods with new ones for the given Tenant Control Plane. Default value is set to Rolling Update, with a blue/green strategy.<br/>
          <br/>
            <i>Default</i>: map[rollingUpdate:map[maxSurge:100% maxUnavailable:0] type:RollingUpdate]<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymenttolerationsindex">tolerations</a></b></td>
        <td>[]object</td>
        <td>
          If specified, the Tenant Control Plane pod's tolerations. More info: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymenttopologyspreadconstraintsindex">topologySpreadConstraints</a></b></td>
        <td>[]object</td>
        <td>
          TopologySpreadConstraints describes how the Tenant Control Plane pods ought to spread across topology domains. Scheduler will schedule pods in a way which abides by the constraints. In case of nil underlying LabelSelector, the Kamaji one for the given Tenant Control Plane will be used. All topologySpreadConstraints are ANDed.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.additionalMetadata



AdditionalMetadata defines which additional metadata, such as labels and annotations, must be attached to the created resource.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>annotations</b></td>
        <td>map[string]string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>labels</b></td>
        <td>map[string]string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity



If specified, the Tenant Control Plane pod's scheduling constraints. More info: https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitynodeaffinity">nodeAffinity</a></b></td>
        <td>object</td>
        <td>
          Describes node affinity scheduling rules for the pod.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodaffinity">podAffinity</a></b></td>
        <td>object</td>
        <td>
          Describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s)).<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodantiaffinity">podAntiAffinity</a></b></td>
        <td>object</td>
        <td>
          Describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s)).<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.nodeAffinity



Describes node affinity scheduling rules for the pod.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitynodeaffinitypreferredduringschedulingignoredduringexecutionindex">preferredDuringSchedulingIgnoredDuringExecution</a></b></td>
        <td>[]object</td>
        <td>
          The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding "weight" to the sum if the node matches the corresponding matchExpressions; the node(s) with the highest sum are the most preferred.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitynodeaffinityrequiredduringschedulingignoredduringexecution">requiredDuringSchedulingIgnoredDuringExecution</a></b></td>
        <td>object</td>
        <td>
          If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[index]



An empty preferred scheduling term matches all objects with implicit weight 0 (i.e. it's a no-op). A null preferred scheduling term matches no objects (i.e. is also a no-op).

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitynodeaffinitypreferredduringschedulingignoredduringexecutionindexpreference">preference</a></b></td>
        <td>object</td>
        <td>
          A node selector term, associated with the corresponding weight.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>weight</b></td>
        <td>integer</td>
        <td>
          Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100.<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[index].preference



A node selector term, associated with the corresponding weight.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitynodeaffinitypreferredduringschedulingignoredduringexecutionindexpreferencematchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          A list of node selector requirements by node's labels.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitynodeaffinitypreferredduringschedulingignoredduringexecutionindexpreferencematchfieldsindex">matchFields</a></b></td>
        <td>[]object</td>
        <td>
          A list of node selector requirements by node's fields.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[index].preference.matchExpressions[index]



A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          The label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[index].preference.matchFields[index]



A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          The label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution



If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to an update), the system may or may not try to eventually evict the pod from its node.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitynodeaffinityrequiredduringschedulingignoredduringexecutionnodeselectortermsindex">nodeSelectorTerms</a></b></td>
        <td>[]object</td>
        <td>
          Required. A list of node selector terms. The terms are ORed.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[index]



A null or empty node selector term matches no objects. The requirements of them are ANDed. The TopologySelectorTerm type implements a subset of the NodeSelectorTerm.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitynodeaffinityrequiredduringschedulingignoredduringexecutionnodeselectortermsindexmatchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          A list of node selector requirements by node's labels.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitynodeaffinityrequiredduringschedulingignoredduringexecutionnodeselectortermsindexmatchfieldsindex">matchFields</a></b></td>
        <td>[]object</td>
        <td>
          A list of node selector requirements by node's fields.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[index].matchExpressions[index]



A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          The label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[index].matchFields[index]



A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          The label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          Represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAffinity



Describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s)).

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodaffinitypreferredduringschedulingignoredduringexecutionindex">preferredDuringSchedulingIgnoredDuringExecution</a></b></td>
        <td>[]object</td>
        <td>
          The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding "weight" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodaffinityrequiredduringschedulingignoredduringexecutionindex">requiredDuringSchedulingIgnoredDuringExecution</a></b></td>
        <td>[]object</td>
        <td>
          If the affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[index]



The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodaffinitypreferredduringschedulingignoredduringexecutionindexpodaffinityterm">podAffinityTerm</a></b></td>
        <td>object</td>
        <td>
          Required. A pod affinity term, associated with the corresponding weight.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>weight</b></td>
        <td>integer</td>
        <td>
          weight associated with matching the corresponding podAffinityTerm, in the range 1-100.<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[index].podAffinityTerm



Required. A pod affinity term, associated with the corresponding weight.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>topologyKey</b></td>
        <td>string</td>
        <td>
          This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodaffinitypreferredduringschedulingignoredduringexecutionindexpodaffinitytermlabelselector">labelSelector</a></b></td>
        <td>object</td>
        <td>
          A label query over a set of resources, in this case pods.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodaffinitypreferredduringschedulingignoredduringexecutionindexpodaffinitytermnamespaceselector">namespaceSelector</a></b></td>
        <td>object</td>
        <td>
          A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespaces</b></td>
        <td>[]string</td>
        <td>
          namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[index].podAffinityTerm.labelSelector



A label query over a set of resources, in this case pods.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodaffinitypreferredduringschedulingignoredduringexecutionindexpodaffinitytermlabelselectormatchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          matchExpressions is a list of label selector requirements. The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchLabels</b></td>
        <td>map[string]string</td>
        <td>
          matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[index].podAffinityTerm.labelSelector.matchExpressions[index]



A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key is the label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[index].podAffinityTerm.namespaceSelector



A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodaffinitypreferredduringschedulingignoredduringexecutionindexpodaffinitytermnamespaceselectormatchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          matchExpressions is a list of label selector requirements. The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchLabels</b></td>
        <td>map[string]string</td>
        <td>
          matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[index].podAffinityTerm.namespaceSelector.matchExpressions[index]



A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key is the label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[index]



Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>topologyKey</b></td>
        <td>string</td>
        <td>
          This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodaffinityrequiredduringschedulingignoredduringexecutionindexlabelselector">labelSelector</a></b></td>
        <td>object</td>
        <td>
          A label query over a set of resources, in this case pods.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodaffinityrequiredduringschedulingignoredduringexecutionindexnamespaceselector">namespaceSelector</a></b></td>
        <td>object</td>
        <td>
          A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespaces</b></td>
        <td>[]string</td>
        <td>
          namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[index].labelSelector



A label query over a set of resources, in this case pods.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodaffinityrequiredduringschedulingignoredduringexecutionindexlabelselectormatchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          matchExpressions is a list of label selector requirements. The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchLabels</b></td>
        <td>map[string]string</td>
        <td>
          matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[index].labelSelector.matchExpressions[index]



A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key is the label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[index].namespaceSelector



A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodaffinityrequiredduringschedulingignoredduringexecutionindexnamespaceselectormatchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          matchExpressions is a list of label selector requirements. The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchLabels</b></td>
        <td>map[string]string</td>
        <td>
          matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[index].namespaceSelector.matchExpressions[index]



A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key is the label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAntiAffinity



Describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s)).

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodantiaffinitypreferredduringschedulingignoredduringexecutionindex">preferredDuringSchedulingIgnoredDuringExecution</a></b></td>
        <td>[]object</td>
        <td>
          The scheduler will prefer to schedule pods to nodes that satisfy the anti-affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling anti-affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding "weight" to the sum if the node has pods which matches the corresponding podAffinityTerm; the node(s) with the highest sum are the most preferred.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodantiaffinityrequiredduringschedulingignoredduringexecutionindex">requiredDuringSchedulingIgnoredDuringExecution</a></b></td>
        <td>[]object</td>
        <td>
          If the anti-affinity requirements specified by this field are not met at scheduling time, the pod will not be scheduled onto the node. If the anti-affinity requirements specified by this field cease to be met at some point during pod execution (e.g. due to a pod label update), the system may or may not try to eventually evict the pod from its node. When there are multiple elements, the lists of nodes corresponding to each podAffinityTerm are intersected, i.e. all terms must be satisfied.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[index]



The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodantiaffinitypreferredduringschedulingignoredduringexecutionindexpodaffinityterm">podAffinityTerm</a></b></td>
        <td>object</td>
        <td>
          Required. A pod affinity term, associated with the corresponding weight.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>weight</b></td>
        <td>integer</td>
        <td>
          weight associated with matching the corresponding podAffinityTerm, in the range 1-100.<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[index].podAffinityTerm



Required. A pod affinity term, associated with the corresponding weight.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>topologyKey</b></td>
        <td>string</td>
        <td>
          This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodantiaffinitypreferredduringschedulingignoredduringexecutionindexpodaffinitytermlabelselector">labelSelector</a></b></td>
        <td>object</td>
        <td>
          A label query over a set of resources, in this case pods.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodantiaffinitypreferredduringschedulingignoredduringexecutionindexpodaffinitytermnamespaceselector">namespaceSelector</a></b></td>
        <td>object</td>
        <td>
          A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespaces</b></td>
        <td>[]string</td>
        <td>
          namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[index].podAffinityTerm.labelSelector



A label query over a set of resources, in this case pods.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodantiaffinitypreferredduringschedulingignoredduringexecutionindexpodaffinitytermlabelselectormatchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          matchExpressions is a list of label selector requirements. The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchLabels</b></td>
        <td>map[string]string</td>
        <td>
          matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[index].podAffinityTerm.labelSelector.matchExpressions[index]



A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key is the label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[index].podAffinityTerm.namespaceSelector



A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodantiaffinitypreferredduringschedulingignoredduringexecutionindexpodaffinitytermnamespaceselectormatchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          matchExpressions is a list of label selector requirements. The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchLabels</b></td>
        <td>map[string]string</td>
        <td>
          matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[index].podAffinityTerm.namespaceSelector.matchExpressions[index]



A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key is the label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[index]



Defines a set of pods (namely those matching the labelSelector relative to the given namespace(s)) that this pod should be co-located (affinity) or not co-located (anti-affinity) with, where co-located is defined as running on a node whose value of the label with key <topologyKey> matches that of any node on which a pod of the set of pods is running

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>topologyKey</b></td>
        <td>string</td>
        <td>
          This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching the labelSelector in the specified namespaces, where co-located is defined as running on a node whose value of the label with key topologyKey matches that of any node on which any of the selected pods is running. Empty topologyKey is not allowed.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodantiaffinityrequiredduringschedulingignoredduringexecutionindexlabelselector">labelSelector</a></b></td>
        <td>object</td>
        <td>
          A label query over a set of resources, in this case pods.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodantiaffinityrequiredduringschedulingignoredduringexecutionindexnamespaceselector">namespaceSelector</a></b></td>
        <td>object</td>
        <td>
          A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespaces</b></td>
        <td>[]string</td>
        <td>
          namespaces specifies a static list of namespace names that the term applies to. The term is applied to the union of the namespaces listed in this field and the ones selected by namespaceSelector. null or empty namespaces list and null namespaceSelector means "this pod's namespace".<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[index].labelSelector



A label query over a set of resources, in this case pods.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodantiaffinityrequiredduringschedulingignoredduringexecutionindexlabelselectormatchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          matchExpressions is a list of label selector requirements. The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchLabels</b></td>
        <td>map[string]string</td>
        <td>
          matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[index].labelSelector.matchExpressions[index]



A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key is the label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[index].namespaceSelector



A label query over the set of namespaces that the term applies to. The term is applied to the union of the namespaces selected by this field and the ones listed in the namespaces field. null selector and null or empty namespaces list means "this pod's namespace". An empty selector ({}) matches all namespaces.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentaffinitypodantiaffinityrequiredduringschedulingignoredduringexecutionindexnamespaceselectormatchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          matchExpressions is a list of label selector requirements. The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchLabels</b></td>
        <td>map[string]string</td>
        <td>
          matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[index].namespaceSelector.matchExpressions[index]



A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key is the label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.extraArgs



ExtraArgs allows adding additional arguments to the Control Plane components, such as kube-apiserver, controller-manager, and scheduler.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>apiServer</b></td>
        <td>[]string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>controllerManager</b></td>
        <td>[]string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>kine</b></td>
        <td>[]string</td>
        <td>
          Available only if Kamaji is running using Kine as backing storage.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>scheduler</b></td>
        <td>[]string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.resources



Resources defines the amount of memory and CPU to allocate to each component of the Control Plane (kube-apiserver, controller-manager, and scheduler).

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentresourcesapiserver">apiServer</a></b></td>
        <td>object</td>
        <td>
          ComponentResourceRequirements describes the compute resource requirements.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentresourcescontrollermanager">controllerManager</a></b></td>
        <td>object</td>
        <td>
          ComponentResourceRequirements describes the compute resource requirements.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentresourcesscheduler">scheduler</a></b></td>
        <td>object</td>
        <td>
          ComponentResourceRequirements describes the compute resource requirements.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.resources.apiServer



ComponentResourceRequirements describes the compute resource requirements.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>limits</b></td>
        <td>map[string]int or string</td>
        <td>
          Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>requests</b></td>
        <td>map[string]int or string</td>
        <td>
          Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.resources.controllerManager



ComponentResourceRequirements describes the compute resource requirements.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>limits</b></td>
        <td>map[string]int or string</td>
        <td>
          Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>requests</b></td>
        <td>map[string]int or string</td>
        <td>
          Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.resources.scheduler



ComponentResourceRequirements describes the compute resource requirements.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>limits</b></td>
        <td>map[string]int or string</td>
        <td>
          Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>requests</b></td>
        <td>map[string]int or string</td>
        <td>
          Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.strategy



Strategy describes how to replace existing pods with new ones for the given Tenant Control Plane. Default value is set to Rolling Update, with a blue/green strategy.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymentstrategyrollingupdate">rollingUpdate</a></b></td>
        <td>object</td>
        <td>
          Rolling update config params. Present only if DeploymentStrategyType = RollingUpdate. --- TODO: Update this to follow our convention for oneOf, whatever we decide it to be.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          Type of deployment. Can be "Recreate" or "RollingUpdate". Default is RollingUpdate.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.strategy.rollingUpdate



Rolling update config params. Present only if DeploymentStrategyType = RollingUpdate. --- TODO: Update this to follow our convention for oneOf, whatever we decide it to be.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>maxSurge</b></td>
        <td>int or string</td>
        <td>
          The maximum number of pods that can be scheduled above the desired number of pods. Value can be an absolute number (ex: 5) or a percentage of desired pods (ex: 10%). This can not be 0 if MaxUnavailable is 0. Absolute number is calculated from percentage by rounding up. Defaults to 25%. Example: when this is set to 30%, the new ReplicaSet can be scaled up immediately when the rolling update starts, such that the total number of old and new pods do not exceed 130% of desired pods. Once old pods have been killed, new ReplicaSet can be scaled up further, ensuring that total number of pods running at any time during the update is at most 130% of desired pods.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>maxUnavailable</b></td>
        <td>int or string</td>
        <td>
          The maximum number of pods that can be unavailable during the update. Value can be an absolute number (ex: 5) or a percentage of desired pods (ex: 10%). Absolute number is calculated from percentage by rounding down. This can not be 0 if MaxSurge is 0. Defaults to 25%. Example: when this is set to 30%, the old ReplicaSet can be scaled down to 70% of desired pods immediately when the rolling update starts. Once new pods are ready, old ReplicaSet can be scaled down further, followed by scaling up the new ReplicaSet, ensuring that the total number of pods available at all times during the update is at least 70% of desired pods.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.tolerations[index]



The pod this Toleration is attached to tolerates any taint that matches the triple <key,value,effect> using the matching operator <operator>.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>effect</b></td>
        <td>string</td>
        <td>
          Effect indicates the taint effect to match. Empty means match all taint effects. When specified, allowed values are NoSchedule, PreferNoSchedule and NoExecute.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key is the taint key that the toleration applies to. Empty means match all taint keys. If the key is empty, operator must be Exists; this combination means to match all values and all keys.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          Operator represents a key's relationship to the value. Valid operators are Exists and Equal. Defaults to Equal. Exists is equivalent to wildcard for value, so that a pod can tolerate all taints of a particular category.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>tolerationSeconds</b></td>
        <td>integer</td>
        <td>
          TolerationSeconds represents the period of time the toleration (which must be of effect NoExecute, otherwise this field is ignored) tolerates the taint. By default, it is not set, which means tolerate the taint forever (do not evict). Zero and negative values will be treated as 0 (evict immediately) by the system.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value is the taint value the toleration matches to. If the operator is Exists, the value should be empty, otherwise just a regular string.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.topologySpreadConstraints[index]



TopologySpreadConstraint specifies how to spread matching pods among the given topology.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>maxSkew</b></td>
        <td>integer</td>
        <td>
          MaxSkew describes the degree to which pods may be unevenly distributed. When `whenUnsatisfiable=DoNotSchedule`, it is the maximum permitted difference between the number of matching pods in the target topology and the global minimum. The global minimum is the minimum number of matching pods in an eligible domain or zero if the number of eligible domains is less than MinDomains. For example, in a 3-zone cluster, MaxSkew is set to 1, and pods with the same labelSelector spread as 2/2/1: In this case, the global minimum is 1. | zone1 | zone2 | zone3 | |  P P  |  P P  |   P   | - if MaxSkew is 1, incoming pod can only be scheduled to zone3 to become 2/2/2; scheduling it onto zone1(zone2) would make the ActualSkew(3-1) on zone1(zone2) violate MaxSkew(1). - if MaxSkew is 2, incoming pod can be scheduled onto any zone. When `whenUnsatisfiable=ScheduleAnyway`, it is used to give higher precedence to topologies that satisfy it. It's a required field. Default value is 1 and 0 is not allowed.<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>topologyKey</b></td>
        <td>string</td>
        <td>
          TopologyKey is the key of node labels. Nodes that have a label with this key and identical values are considered to be in the same topology. We consider each <key, value> as a "bucket", and try to put balanced number of pods into each bucket. We define a domain as a particular instance of a topology. Also, we define an eligible domain as a domain whose nodes meet the requirements of nodeAffinityPolicy and nodeTaintsPolicy. e.g. If TopologyKey is "kubernetes.io/hostname", each Node is a domain of that topology. And, if TopologyKey is "topology.kubernetes.io/zone", each zone is a domain of that topology. It's a required field.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>whenUnsatisfiable</b></td>
        <td>string</td>
        <td>
          WhenUnsatisfiable indicates how to deal with a pod if it doesn't satisfy the spread constraint. - DoNotSchedule (default) tells the scheduler not to schedule it. - ScheduleAnyway tells the scheduler to schedule the pod in any location, but giving higher precedence to topologies that would help reduce the skew. A constraint is considered "Unsatisfiable" for an incoming pod if and only if every possible node assignment for that pod would violate "MaxSkew" on some topology. For example, in a 3-zone cluster, MaxSkew is set to 1, and pods with the same labelSelector spread as 3/1/1: | zone1 | zone2 | zone3 | | P P P |   P   |   P   | If WhenUnsatisfiable is set to DoNotSchedule, incoming pod can only be scheduled to zone2(zone3) to become 3/2/1(3/1/2) as ActualSkew(2-1) on zone2(zone3) satisfies MaxSkew(1). In other words, the cluster can still be imbalanced, but scheduler won't make it *more* imbalanced. It's a required field.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymenttopologyspreadconstraintsindexlabelselector">labelSelector</a></b></td>
        <td>object</td>
        <td>
          LabelSelector is used to find matching pods. Pods that match this label selector are counted to determine the number of pods in their corresponding topology domain.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchLabelKeys</b></td>
        <td>[]string</td>
        <td>
          MatchLabelKeys is a set of pod label keys to select the pods over which spreading will be calculated. The keys are used to lookup values from the incoming pod labels, those key-value labels are ANDed with labelSelector to select the group of existing pods over which spreading will be calculated for the incoming pod. Keys that don't exist in the incoming pod labels will be ignored. A null or empty list means only match against labelSelector.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>minDomains</b></td>
        <td>integer</td>
        <td>
          MinDomains indicates a minimum number of eligible domains. When the number of eligible domains with matching topology keys is less than minDomains, Pod Topology Spread treats "global minimum" as 0, and then the calculation of Skew is performed. And when the number of eligible domains with matching topology keys equals or greater than minDomains, this value has no effect on scheduling. As a result, when the number of eligible domains is less than minDomains, scheduler won't schedule more than maxSkew Pods to those domains. If value is nil, the constraint behaves as if MinDomains is equal to 1. Valid values are integers greater than 0. When value is not nil, WhenUnsatisfiable must be DoNotSchedule. 
 For example, in a 3-zone cluster, MaxSkew is set to 2, MinDomains is set to 5 and pods with the same labelSelector spread as 2/2/2: | zone1 | zone2 | zone3 | |  P P  |  P P  |  P P  | The number of domains is less than 5(MinDomains), so "global minimum" is treated as 0. In this situation, new pod with the same labelSelector cannot be scheduled, because computed skew will be 3(3 - 0) if new Pod is scheduled to any of the three zones, it will violate MaxSkew. 
 This is a beta field and requires the MinDomainsInPodTopologySpread feature gate to be enabled (enabled by default).<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>nodeAffinityPolicy</b></td>
        <td>string</td>
        <td>
          NodeAffinityPolicy indicates how we will treat Pod's nodeAffinity/nodeSelector when calculating pod topology spread skew. Options are: - Honor: only nodes matching nodeAffinity/nodeSelector are included in the calculations. - Ignore: nodeAffinity/nodeSelector are ignored. All nodes are included in the calculations. 
 If this value is nil, the behavior is equivalent to the Honor policy. This is a beta-level feature default enabled by the NodeInclusionPolicyInPodTopologySpread feature flag.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>nodeTaintsPolicy</b></td>
        <td>string</td>
        <td>
          NodeTaintsPolicy indicates how we will treat node taints when calculating pod topology spread skew. Options are: - Honor: nodes without taints, along with tainted nodes for which the incoming pod has a toleration, are included. - Ignore: node taints are ignored. All nodes are included. 
 If this value is nil, the behavior is equivalent to the Ignore policy. This is a beta-level feature default enabled by the NodeInclusionPolicyInPodTopologySpread feature flag.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.topologySpreadConstraints[index].labelSelector



LabelSelector is used to find matching pods. Pods that match this label selector are counted to determine the number of pods in their corresponding topology domain.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplanedeploymenttopologyspreadconstraintsindexlabelselectormatchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          matchExpressions is a list of label selector requirements. The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchLabels</b></td>
        <td>map[string]string</td>
        <td>
          matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value". The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.deployment.topologySpreadConstraints[index].labelSelector.matchExpressions[index]



A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key is the label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          operator represents a key's relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.ingress



Defining the options for an Optional Ingress which will expose API Server of the Tenant Control Plane

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeccontrolplaneingressadditionalmetadata">additionalMetadata</a></b></td>
        <td>object</td>
        <td>
          AdditionalMetadata defines which additional metadata, such as labels and annotations, must be attached to the created resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>hostname</b></td>
        <td>string</td>
        <td>
          Hostname is an optional field which will be used as Ingress's Host. If it is not defined, Ingress's host will be "<tenant>.<namespace>.<domain>", where domain is specified under NetworkProfileSpec<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>ingressClassName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.controlPlane.ingress.additionalMetadata



AdditionalMetadata defines which additional metadata, such as labels and annotations, must be attached to the created resource.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>annotations</b></td>
        <td>map[string]string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>labels</b></td>
        <td>map[string]string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.kubernetes



Kubernetes specification for tenant control plane

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespeckuberneteskubelet">kubelet</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>version</b></td>
        <td>string</td>
        <td>
          Kubernetes Version for the tenant control plane<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>admissionControllers</b></td>
        <td>[]enum</td>
        <td>
          List of enabled Admission Controllers for the Tenant cluster. Full reference available here: https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers<br/>
          <br/>
            <i>Default</i>: [CertificateApproval CertificateSigning CertificateSubjectRestriction DefaultIngressClass DefaultStorageClass DefaultTolerationSeconds LimitRanger MutatingAdmissionWebhook NamespaceLifecycle PersistentVolumeClaimResize Priority ResourceQuota RuntimeClass ServiceAccount StorageObjectInUseProtection TaintNodesByCondition ValidatingAdmissionWebhook]<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.kubernetes.kubelet





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>cgroupfs</b></td>
        <td>enum</td>
        <td>
          CGroupFS defines the  cgroup driver for Kubelet https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/configure-cgroup-driver/<br/>
          <br/>
            <i>Enum</i>: systemd, cgroupfs<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>preferredAddressTypes</b></td>
        <td>[]enum</td>
        <td>
          Ordered list of the preferred NodeAddressTypes to use for kubelet connections. Default to Hostname, InternalIP, ExternalIP.<br/>
          <br/>
            <i>Default</i>: [Hostname InternalIP ExternalIP]<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.addons



Addons contain which addons are enabled

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespecaddonscoredns">coreDNS</a></b></td>
        <td>object</td>
        <td>
          Enables the DNS addon in the Tenant Cluster. The registry and the tag are configurable, the image is hard-coded to `coredns`.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespecaddonskonnectivity">konnectivity</a></b></td>
        <td>object</td>
        <td>
          Enables the Konnectivity addon in the Tenant Cluster, required if the worker nodes are in a different network.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespecaddonskubeproxy">kubeProxy</a></b></td>
        <td>object</td>
        <td>
          Enables the kube-proxy addon in the Tenant Cluster. The registry and the tag are configurable, the image is hard-coded to `kube-proxy`.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.addons.coreDNS



Enables the DNS addon in the Tenant Cluster. The registry and the tag are configurable, the image is hard-coded to `coredns`.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>imageRepository</b></td>
        <td>string</td>
        <td>
          ImageRepository sets the container registry to pull images from. if not set, the default ImageRepository will be used instead.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>imageTag</b></td>
        <td>string</td>
        <td>
          ImageTag allows to specify a tag for the image. In case this value is set, kubeadm does not change automatically the version of the above components during upgrades.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.addons.konnectivity



Enables the Konnectivity addon in the Tenant Cluster, required if the worker nodes are in a different network.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanespecaddonskonnectivityagent">agent</a></b></td>
        <td>object</td>
        <td>
          <br/>
          <br/>
            <i>Default</i>: map[image:registry.k8s.io/kas-network-proxy/proxy-agent version:v0.0.32]<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespecaddonskonnectivityserver">server</a></b></td>
        <td>object</td>
        <td>
          <br/>
          <br/>
            <i>Default</i>: map[image:registry.k8s.io/kas-network-proxy/proxy-server port:8132 version:v0.0.32]<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.addons.konnectivity.agent





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>extraArgs</b></td>
        <td>[]string</td>
        <td>
          ExtraArgs allows adding additional arguments to said component.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>image</b></td>
        <td>string</td>
        <td>
          AgentImage defines the container image for Konnectivity's agent.<br/>
          <br/>
            <i>Default</i>: registry.k8s.io/kas-network-proxy/proxy-agent<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>version</b></td>
        <td>string</td>
        <td>
          Version for Konnectivity agent.<br/>
          <br/>
            <i>Default</i>: v0.0.32<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.addons.konnectivity.server





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>port</b></td>
        <td>integer</td>
        <td>
          The port which Konnectivity server is listening to.<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>extraArgs</b></td>
        <td>[]string</td>
        <td>
          ExtraArgs allows adding additional arguments to said component.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>image</b></td>
        <td>string</td>
        <td>
          Container image used by the Konnectivity server.<br/>
          <br/>
            <i>Default</i>: registry.k8s.io/kas-network-proxy/proxy-server<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanespecaddonskonnectivityserverresources">resources</a></b></td>
        <td>object</td>
        <td>
          Resources define the amount of CPU and memory to allocate to the Konnectivity server.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>version</b></td>
        <td>string</td>
        <td>
          Container image version of the Konnectivity server.<br/>
          <br/>
            <i>Default</i>: v0.0.32<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.addons.konnectivity.server.resources



Resources define the amount of CPU and memory to allocate to the Konnectivity server.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>limits</b></td>
        <td>map[string]int or string</td>
        <td>
          Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>requests</b></td>
        <td>map[string]int or string</td>
        <td>
          Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.addons.kubeProxy



Enables the kube-proxy addon in the Tenant Cluster. The registry and the tag are configurable, the image is hard-coded to `kube-proxy`.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>imageRepository</b></td>
        <td>string</td>
        <td>
          ImageRepository sets the container registry to pull images from. if not set, the default ImageRepository will be used instead.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>imageTag</b></td>
        <td>string</td>
        <td>
          ImageTag allows to specify a tag for the image. In case this value is set, kubeadm does not change automatically the version of the above components during upgrades.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.spec.networkProfile



NetworkProfile specifies how the network is

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>address</b></td>
        <td>string</td>
        <td>
          Address where API server of will be exposed. In case of LoadBalancer Service, this can be empty in order to use the exposed IP provided by the cloud controller manager.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>allowAddressAsExternalIP</b></td>
        <td>boolean</td>
        <td>
          AllowAddressAsExternalIP will include tenantControlPlane.Spec.NetworkProfile.Address in the section of ExternalIPs of the Kubernetes Service (only ClusterIP or NodePort)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>certSANs</b></td>
        <td>[]string</td>
        <td>
          CertSANs sets extra Subject Alternative Names (SANs) for the API Server signing certificate. Use this field to add additional hostnames when exposing the Tenant Control Plane with third solutions.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>dnsServiceIPs</b></td>
        <td>[]string</td>
        <td>
          <br/>
          <br/>
            <i>Default</i>: [10.96.0.10]<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>podCidr</b></td>
        <td>string</td>
        <td>
          CIDR for Kubernetes Pods<br/>
          <br/>
            <i>Default</i>: 10.244.0.0/16<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>port</b></td>
        <td>integer</td>
        <td>
          Port where API server of will be exposed<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Default</i>: 6443<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>serviceCidr</b></td>
        <td>string</td>
        <td>
          Kubernetes Service<br/>
          <br/>
            <i>Default</i>: 10.96.0.0/16<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status



TenantControlPlaneStatus defines the observed state of TenantControlPlane.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanestatusaddons">addons</a></b></td>
        <td>object</td>
        <td>
          Addons contains the status of the different Addons<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuscertificates">certificates</a></b></td>
        <td>object</td>
        <td>
          Certificates contains information about the different certificates that are necessary to run a kubernetes control plane<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>controlPlaneEndpoint</b></td>
        <td>string</td>
        <td>
          ControlPlaneEndpoint contains the status of the kubernetes control plane<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuskubeadmphase">kubeadmPhase</a></b></td>
        <td>object</td>
        <td>
          KubeadmPhase contains the status of the kubeadm phases action<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuskubeadmconfig">kubeadmconfig</a></b></td>
        <td>object</td>
        <td>
          KubeadmConfig contains the status of the configuration required by kubeadm<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuskubeconfig">kubeconfig</a></b></td>
        <td>object</td>
        <td>
          KubeConfig contains information about the kubenconfigs that control plane pieces need<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuskubernetesresources">kubernetesResources</a></b></td>
        <td>object</td>
        <td>
          Kubernetes contains information about the reconciliation of the required Kubernetes resources deployed in the admin cluster<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatusstorage">storage</a></b></td>
        <td>object</td>
        <td>
          Storage Status contains information about Kubernetes storage system<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.addons



Addons contains the status of the different Addons

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanestatusaddonscoredns">coreDNS</a></b></td>
        <td>object</td>
        <td>
          AddonStatus defines the observed state of an Addon.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatusaddonskonnectivity">konnectivity</a></b></td>
        <td>object</td>
        <td>
          KonnectivityStatus defines the status of Konnectivity as Addon.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatusaddonskubeproxy">kubeProxy</a></b></td>
        <td>object</td>
        <td>
          AddonStatus defines the observed state of an Addon.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.addons.coreDNS



AddonStatus defines the observed state of an Addon.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>enabled</b></td>
        <td>boolean</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.addons.konnectivity



KonnectivityStatus defines the status of Konnectivity as Addon.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>enabled</b></td>
        <td>boolean</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatusaddonskonnectivityagent">agent</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatusaddonskonnectivitycertificate">certificate</a></b></td>
        <td>object</td>
        <td>
          CertificatePrivateKeyPairStatus defines the status.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatusaddonskonnectivityclusterrolebinding">clusterrolebinding</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatusaddonskonnectivityconfigmap">configMap</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatusaddonskonnectivitykubeconfig">kubeconfig</a></b></td>
        <td>object</td>
        <td>
          KubeconfigStatus contains information about the generated kubeconfig.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatusaddonskonnectivitysa">sa</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatusaddonskonnectivityservice">service</a></b></td>
        <td>object</td>
        <td>
          KubernetesServiceStatus defines the status for the Tenant Control Plane Service in the management cluster.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.addons.konnectivity.agent





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          Last time when k8s object was updated<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.addons.konnectivity.certificate



CertificatePrivateKeyPairStatus defines the status.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secretName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.addons.konnectivity.clusterrolebinding





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          Last time when k8s object was updated<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.addons.konnectivity.configMap





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.addons.konnectivity.kubeconfig



KubeconfigStatus contains information about the generated kubeconfig.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secretName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.addons.konnectivity.sa





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          Last time when k8s object was updated<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.addons.konnectivity.service



KubernetesServiceStatus defines the status for the Tenant Control Plane Service in the management cluster.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          The name of the Service for the given cluster.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          The namespace which the Service for the given cluster is deployed.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>port</b></td>
        <td>integer</td>
        <td>
          The port where the service is running<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatusaddonskonnectivityserviceconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Current service state<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatusaddonskonnectivityserviceloadbalancer">loadBalancer</a></b></td>
        <td>object</td>
        <td>
          LoadBalancer contains the current status of the load-balancer, if one is present.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.addons.konnectivity.service.conditions[index]



Condition contains details for one aspect of the current state of this API Resource. --- This struct is intended for direct use as an array at the field path .status.conditions.  For example, 
 type FooStatus struct{ // Represents the observations of a foo's current state. // Known .status.conditions.type are: "Available", "Progressing", and "Degraded" // +patchMergeKey=type // +patchStrategy=merge // +listType=map // +listMapKey=type Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"` 
 // other fields }

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastTransitionTime</b></td>
        <td>string</td>
        <td>
          lastTransitionTime is the last time the condition transitioned from one status to another. This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          message is a human readable message indicating details about the transition. This may be an empty string.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          reason contains a programmatic identifier indicating the reason for the condition's last transition. Producers of specific condition types may define expected values and meanings for this field, and whether the values are considered a guaranteed API. The value should be a CamelCase string. This field may not be empty.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>enum</td>
        <td>
          status of the condition, one of True, False, Unknown.<br/>
          <br/>
            <i>Enum</i>: True, False, Unknown<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          type of condition in CamelCase or in foo.example.com/CamelCase. --- Many .condition.type values are consistent across resources like Available, but because arbitrary conditions can be useful (see .node.status.conditions), the ability to deconflict is important. The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          observedGeneration represents the .metadata.generation that the condition was set based upon. For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date with respect to the current state of the instance.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.addons.konnectivity.service.loadBalancer



LoadBalancer contains the current status of the load-balancer, if one is present.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanestatusaddonskonnectivityserviceloadbalanceringressindex">ingress</a></b></td>
        <td>[]object</td>
        <td>
          Ingress is a list containing ingress points for the load-balancer. Traffic intended for the service should be sent to these ingress points.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.addons.konnectivity.service.loadBalancer.ingress[index]



LoadBalancerIngress represents the status of a load-balancer ingress point: traffic intended for the service should be sent to an ingress point.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>hostname</b></td>
        <td>string</td>
        <td>
          Hostname is set for load-balancer ingress points that are DNS based (typically AWS load-balancers)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>ip</b></td>
        <td>string</td>
        <td>
          IP is set for load-balancer ingress points that are IP based (typically GCE or OpenStack load-balancers)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatusaddonskonnectivityserviceloadbalanceringressindexportsindex">ports</a></b></td>
        <td>[]object</td>
        <td>
          Ports is a list of records of service ports If used, every port defined in the service should have an entry in it<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.addons.konnectivity.service.loadBalancer.ingress[index].ports[index]





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>port</b></td>
        <td>integer</td>
        <td>
          Port is the port number of the service port of which status is recorded here<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>protocol</b></td>
        <td>string</td>
        <td>
          Protocol is the protocol of the service port of which status is recorded here The supported values are: "TCP", "UDP", "SCTP"<br/>
          <br/>
            <i>Default</i>: TCP<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>error</b></td>
        <td>string</td>
        <td>
          Error is to record the problem with the service port The format of the error shall comply with the following rules: - built-in error values shall be specified in this file and those shall use CamelCase names - cloud provider specific error values must have names that comply with the format foo.example.com/CamelCase. --- The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.addons.kubeProxy



AddonStatus defines the observed state of an Addon.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>enabled</b></td>
        <td>boolean</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.certificates



Certificates contains information about the different certificates that are necessary to run a kubernetes control plane

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanestatuscertificatesapiserver">apiServer</a></b></td>
        <td>object</td>
        <td>
          CertificatePrivateKeyPairStatus defines the status.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuscertificatesapiserverkubeletclient">apiServerKubeletClient</a></b></td>
        <td>object</td>
        <td>
          CertificatePrivateKeyPairStatus defines the status.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuscertificatesca">ca</a></b></td>
        <td>object</td>
        <td>
          CertificatePrivateKeyPairStatus defines the status.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuscertificatesetcd">etcd</a></b></td>
        <td>object</td>
        <td>
          ETCDCertificatesStatus defines the observed state of ETCD Certificate for API server.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuscertificatesfrontproxyca">frontProxyCA</a></b></td>
        <td>object</td>
        <td>
          CertificatePrivateKeyPairStatus defines the status.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuscertificatesfrontproxyclient">frontProxyClient</a></b></td>
        <td>object</td>
        <td>
          CertificatePrivateKeyPairStatus defines the status.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuscertificatessa">sa</a></b></td>
        <td>object</td>
        <td>
          PublicKeyPrivateKeyPairStatus defines the status.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.certificates.apiServer



CertificatePrivateKeyPairStatus defines the status.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secretName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.certificates.apiServerKubeletClient



CertificatePrivateKeyPairStatus defines the status.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secretName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.certificates.ca



CertificatePrivateKeyPairStatus defines the status.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secretName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.certificates.etcd



ETCDCertificatesStatus defines the observed state of ETCD Certificate for API server.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanestatuscertificatesetcdapiserver">apiServer</a></b></td>
        <td>object</td>
        <td>
          APIServerCertificatesStatus defines the observed state of ETCD Certificate for API server.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuscertificatesetcdca">ca</a></b></td>
        <td>object</td>
        <td>
          ETCDCertificateStatus defines the observed state of ETCD Certificate for API server.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.certificates.etcd.apiServer



APIServerCertificatesStatus defines the observed state of ETCD Certificate for API server.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secretName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.certificates.etcd.ca



ETCDCertificateStatus defines the observed state of ETCD Certificate for API server.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secretName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.certificates.frontProxyCA



CertificatePrivateKeyPairStatus defines the status.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secretName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.certificates.frontProxyClient



CertificatePrivateKeyPairStatus defines the status.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secretName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.certificates.sa



PublicKeyPrivateKeyPairStatus defines the status.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secretName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubeadmPhase



KubeadmPhase contains the status of the kubeadm phases action

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanestatuskubeadmphasebootstraptoken">bootstrapToken</a></b></td>
        <td>object</td>
        <td>
          KubeadmPhaseStatus contains the status of a kubeadm phase action.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubeadmPhase.bootstrapToken



KubeadmPhaseStatus contains the status of a kubeadm phase action.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubeadmconfig



KubeadmConfig contains the status of the configuration required by kubeadm

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          Checksum of the kubeadm configuration to detect changes<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>configmapName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubeconfig



KubeConfig contains information about the kubenconfigs that control plane pieces need

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanestatuskubeconfigadmin">admin</a></b></td>
        <td>object</td>
        <td>
          KubeconfigStatus contains information about the generated kubeconfig.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuskubeconfigcontrollermanager">controllerManager</a></b></td>
        <td>object</td>
        <td>
          KubeconfigStatus contains information about the generated kubeconfig.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuskubeconfigscheduler">scheduler</a></b></td>
        <td>object</td>
        <td>
          KubeconfigStatus contains information about the generated kubeconfig.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubeconfig.admin



KubeconfigStatus contains information about the generated kubeconfig.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secretName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubeconfig.controllerManager



KubeconfigStatus contains information about the generated kubeconfig.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secretName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubeconfig.scheduler



KubeconfigStatus contains information about the generated kubeconfig.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secretName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubernetesResources



Kubernetes contains information about the reconciliation of the required Kubernetes resources deployed in the admin cluster

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanestatuskubernetesresourcesdeployment">deployment</a></b></td>
        <td>object</td>
        <td>
          KubernetesDeploymentStatus defines the status for the Tenant Control Plane Deployment in the management cluster.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuskubernetesresourcesingress">ingress</a></b></td>
        <td>object</td>
        <td>
          KubernetesIngressStatus defines the status for the Tenant Control Plane Ingress in the management cluster.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuskubernetesresourcesservice">service</a></b></td>
        <td>object</td>
        <td>
          KubernetesServiceStatus defines the status for the Tenant Control Plane Service in the management cluster.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuskubernetesresourcesversion">version</a></b></td>
        <td>object</td>
        <td>
          KubernetesVersion contains the information regarding the running Kubernetes version, and its upgrade status.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubernetesResources.deployment



KubernetesDeploymentStatus defines the status for the Tenant Control Plane Deployment in the management cluster.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          The name of the Deployment for the given cluster.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          The namespace which the Deployment for the given cluster is deployed.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>selector</b></td>
        <td>string</td>
        <td>
          Selector is the label selector used to group the Tenant Control Plane Pods used by the scale subresource.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>availableReplicas</b></td>
        <td>integer</td>
        <td>
          Total number of available pods (ready for at least minReadySeconds) targeted by this deployment.<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>collisionCount</b></td>
        <td>integer</td>
        <td>
          Count of hash collisions for the Deployment. The Deployment controller uses this field as a collision avoidance mechanism when it needs to create the name for the newest ReplicaSet.<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuskubernetesresourcesdeploymentconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Represents the latest available observations of a deployment's current state.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          Last time when deployment was updated<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          The generation observed by the deployment controller.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>readyReplicas</b></td>
        <td>integer</td>
        <td>
          readyReplicas is the number of pods targeted by this Deployment with a Ready Condition.<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>replicas</b></td>
        <td>integer</td>
        <td>
          Total number of non-terminated pods targeted by this deployment (their labels match the selector).<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>unavailableReplicas</b></td>
        <td>integer</td>
        <td>
          Total number of unavailable pods targeted by this deployment. This is the total number of pods that are still required for the deployment to have 100% available capacity. They may either be pods that are running but not yet available or pods that still have not been created.<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>updatedReplicas</b></td>
        <td>integer</td>
        <td>
          Total number of non-terminated pods targeted by this deployment that have the desired template spec.<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubernetesResources.deployment.conditions[index]



DeploymentCondition describes the state of a deployment at a certain point.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>status</b></td>
        <td>string</td>
        <td>
          Status of the condition, one of True, False, Unknown.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          Type of deployment condition.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>lastTransitionTime</b></td>
        <td>string</td>
        <td>
          Last time the condition transitioned from one status to another.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdateTime</b></td>
        <td>string</td>
        <td>
          The last time this condition was updated.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          A human readable message indicating details about the transition.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          The reason for the condition's last transition.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubernetesResources.ingress



KubernetesIngressStatus defines the status for the Tenant Control Plane Ingress in the management cluster.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          The name of the Ingress for the given cluster.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          The namespace which the Ingress for the given cluster is deployed.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuskubernetesresourcesingressloadbalancer">loadBalancer</a></b></td>
        <td>object</td>
        <td>
          LoadBalancer contains the current status of the load-balancer.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubernetesResources.ingress.loadBalancer



LoadBalancer contains the current status of the load-balancer.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanestatuskubernetesresourcesingressloadbalanceringressindex">ingress</a></b></td>
        <td>[]object</td>
        <td>
          Ingress is a list containing ingress points for the load-balancer.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubernetesResources.ingress.loadBalancer.ingress[index]



IngressLoadBalancerIngress represents the status of a load-balancer ingress point.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>hostname</b></td>
        <td>string</td>
        <td>
          Hostname is set for load-balancer ingress points that are DNS based.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>ip</b></td>
        <td>string</td>
        <td>
          IP is set for load-balancer ingress points that are IP based.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuskubernetesresourcesingressloadbalanceringressindexportsindex">ports</a></b></td>
        <td>[]object</td>
        <td>
          Ports provides information about the ports exposed by this LoadBalancer.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubernetesResources.ingress.loadBalancer.ingress[index].ports[index]



IngressPortStatus represents the error condition of a service port

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>port</b></td>
        <td>integer</td>
        <td>
          Port is the port number of the ingress port.<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>protocol</b></td>
        <td>string</td>
        <td>
          Protocol is the protocol of the ingress port. The supported values are: "TCP", "UDP", "SCTP"<br/>
          <br/>
            <i>Default</i>: TCP<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>error</b></td>
        <td>string</td>
        <td>
          Error is to record the problem with the service port The format of the error shall comply with the following rules: - built-in error values shall be specified in this file and those shall use CamelCase names - cloud provider specific error values must have names that comply with the format foo.example.com/CamelCase. --- The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubernetesResources.service



KubernetesServiceStatus defines the status for the Tenant Control Plane Service in the management cluster.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          The name of the Service for the given cluster.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          The namespace which the Service for the given cluster is deployed.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>port</b></td>
        <td>integer</td>
        <td>
          The port where the service is running<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuskubernetesresourcesserviceconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Current service state<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuskubernetesresourcesserviceloadbalancer">loadBalancer</a></b></td>
        <td>object</td>
        <td>
          LoadBalancer contains the current status of the load-balancer, if one is present.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubernetesResources.service.conditions[index]



Condition contains details for one aspect of the current state of this API Resource. --- This struct is intended for direct use as an array at the field path .status.conditions.  For example, 
 type FooStatus struct{ // Represents the observations of a foo's current state. // Known .status.conditions.type are: "Available", "Progressing", and "Degraded" // +patchMergeKey=type // +patchStrategy=merge // +listType=map // +listMapKey=type Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"` 
 // other fields }

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastTransitionTime</b></td>
        <td>string</td>
        <td>
          lastTransitionTime is the last time the condition transitioned from one status to another. This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          message is a human readable message indicating details about the transition. This may be an empty string.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          reason contains a programmatic identifier indicating the reason for the condition's last transition. Producers of specific condition types may define expected values and meanings for this field, and whether the values are considered a guaranteed API. The value should be a CamelCase string. This field may not be empty.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>enum</td>
        <td>
          status of the condition, one of True, False, Unknown.<br/>
          <br/>
            <i>Enum</i>: True, False, Unknown<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          type of condition in CamelCase or in foo.example.com/CamelCase. --- Many .condition.type values are consistent across resources like Available, but because arbitrary conditions can be useful (see .node.status.conditions), the ability to deconflict is important. The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          observedGeneration represents the .metadata.generation that the condition was set based upon. For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date with respect to the current state of the instance.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubernetesResources.service.loadBalancer



LoadBalancer contains the current status of the load-balancer, if one is present.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanestatuskubernetesresourcesserviceloadbalanceringressindex">ingress</a></b></td>
        <td>[]object</td>
        <td>
          Ingress is a list containing ingress points for the load-balancer. Traffic intended for the service should be sent to these ingress points.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubernetesResources.service.loadBalancer.ingress[index]



LoadBalancerIngress represents the status of a load-balancer ingress point: traffic intended for the service should be sent to an ingress point.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>hostname</b></td>
        <td>string</td>
        <td>
          Hostname is set for load-balancer ingress points that are DNS based (typically AWS load-balancers)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>ip</b></td>
        <td>string</td>
        <td>
          IP is set for load-balancer ingress points that are IP based (typically GCE or OpenStack load-balancers)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatuskubernetesresourcesserviceloadbalanceringressindexportsindex">ports</a></b></td>
        <td>[]object</td>
        <td>
          Ports is a list of records of service ports If used, every port defined in the service should have an entry in it<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubernetesResources.service.loadBalancer.ingress[index].ports[index]





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>port</b></td>
        <td>integer</td>
        <td>
          Port is the port number of the service port of which status is recorded here<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>protocol</b></td>
        <td>string</td>
        <td>
          Protocol is the protocol of the service port of which status is recorded here The supported values are: "TCP", "UDP", "SCTP"<br/>
          <br/>
            <i>Default</i>: TCP<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>error</b></td>
        <td>string</td>
        <td>
          Error is to record the problem with the service port The format of the error shall comply with the following rules: - built-in error values shall be specified in this file and those shall use CamelCase names - cloud provider specific error values must have names that comply with the format foo.example.com/CamelCase. --- The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.kubernetesResources.version



KubernetesVersion contains the information regarding the running Kubernetes version, and its upgrade status.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>status</b></td>
        <td>enum</td>
        <td>
          Status returns the current status of the Kubernetes version, such as its provisioning state, or completed upgrade.<br/>
          <br/>
            <i>Enum</i>: Provisioning, CertificateAuthorityRotating, Upgrading, Migrating, Ready, NotReady<br/>
            <i>Default</i>: Provisioning<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>version</b></td>
        <td>string</td>
        <td>
          Version is the running Kubernetes version of the Tenant Control Plane.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.storage



Storage Status contains information about Kubernetes storage system

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#tenantcontrolplanestatusstoragecertificate">certificate</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatusstorageconfig">config</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>dataStoreName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>driver</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#tenantcontrolplanestatusstoragesetup">setup</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.storage.certificate





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secretName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.storage.config





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>secretName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### TenantControlPlane.status.storage.setup





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdate</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>schema</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>user</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>