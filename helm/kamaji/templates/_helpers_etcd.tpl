{{/*
Create a default fully qualified etcd name.
*/}}
{{- define "etcd.fullname" -}}
{{- printf "etcd" }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "etcd.serviceAccountName" -}}
{{- if .Values.etcd.serviceAccount.create }}
{{- default (include "etcd.fullname" .) .Values.etcd.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.etcd.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the Service to use
*/}}
{{- define "etcd.serviceName" -}}
{{- printf "%s" (include "etcd.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "etcd.labels" -}}
app.kubernetes.io/name: {{ include "kamaji.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/components: etcd
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "etcd.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kamaji.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: etcd
{{- end }}

{{/*
Name of the etcd CA secret.
*/}}
{{- define "etcd.caSecretName" }}
{{- if .Values.etcd.deploy }}
{{- printf "%s-%s" (include "etcd.fullname" .) "certs" | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- required "A valid .Values.etcd.overrides.caSecret.name required!" .Values.etcd.overrides.caSecret.name }}
{{- end }}
{{- end }}

{{/*
Namespace of the etcd CA secret.
*/}}
{{- define "etcd.caSecretNamespace" }}
{{- if .Values.etcd.deploy }}
{{- .Release.Namespace }}
{{- else }}
{{- required "A valid .Values.etcd.overrides.caSecret.namespace required!" .Values.etcd.overrides.caSecret.namespace }}
{{- end }}
{{- end }}

{{/*
Name of the certificate signing requests for the certificates required by etcd.
*/}}
{{- define "etcd.csrConfigMapName" }}
{{- printf "%s-csr" (include "etcd.fullname" .) }}
{{- end }}

{{/*
Name of the etcd root-client secret.
*/}}
{{- define "etcd.clientSecretName" }}
{{- if .Values.etcd.deploy }}
{{- printf "root-client-certs" }}
{{- else }}
{{- required "A valid .Values.etcd.overrides.clientSecret.name required!" .Values.etcd.overrides.clientSecret.name }}
{{- end }}
{{- end }}

{{/*
Namespace of the etcd root-client secret.
*/}}
{{- define "etcd.clientSecretNamespace" }}
{{- if .Values.etcd.deploy }}
{{- .Release.Namespace }}
{{- else }}
{{- required "A valid .Values.etcd.overrides.clientSecret.namespace required!" .Values.etcd.overrides.clientSecret.namespace }}
{{- end }}
{{- end }}

{{/*
List the declared etcd endpoints, using the overrides in case of unmanaged etcd.
*/}}
{{- define "etcd.endpoints" }}
{{- if .Values.etcd.deploy }}
{{- range $count := until 3 -}}
    {{- printf "https://%s-%d.%s.%s.svc.cluster.local:2379" "etcd" $count ( include "etcd.serviceName" . ) $.Release.Namespace -}}
    {{- if lt $count  ( sub 3 1 ) -}}
      {{- printf "," -}}
    {{- end -}}
{{- end }}
{{- else }}
{{- required "A valid .Values.etcd.overrides.endpoints required!" .Values.etcd.overrides.endpoints }}
{{- end }}
{{- end }}

{{/*
Retrieve the current Kubernetes version to launch a kubectl container with the minimum version skew possible.
*/}}
{{- define "etcd.jobsTagKubeVersion" -}}
{{- if contains "-eks-" .Capabilities.KubeVersion.GitVersion }}
{{- print "v" .Capabilities.KubeVersion.Major "." (.Capabilities.KubeVersion.Minor | replace "+" "") -}}
{{- else }}
{{- print "v" .Capabilities.KubeVersion.Major "." .Capabilities.KubeVersion.Minor -}}
{{- end }}
{{- end }}
