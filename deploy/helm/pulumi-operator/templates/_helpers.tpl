{{/*
Expand the name of the chart.
*/}}
{{- define "pulumi-kubernetes-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "pulumi-kubernetes-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "pulumi-kubernetes-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "pulumi-kubernetes-operator.labels" -}}
helm.sh/chart: {{ include "pulumi-kubernetes-operator.chart" . }}
{{ include "pulumi-kubernetes-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "pulumi-kubernetes-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pulumi-kubernetes-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "pulumi-kubernetes-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "pulumi-kubernetes-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the advertised address for the controller
*/}}
{{- define "pulumi-kubernetes-operator.advertisedAddress" -}}
{{- if .Values.controller.advertisedAddress }}
{{- .Values.controller.advertisedAddress }}
{{- else }}
{{- include "pulumi-kubernetes-operator.fullname" . }}.$(POD_NAMESPACE)
{{- end }}
{{- end }}


{{/*
Create the name of the image to use
*/}}
{{- define "pulumi-kubernetes-operator.imageName" -}}
{{- .Values.image.registry }}/{{- .Values.image.repository }}:{{- .Values.image.tag | default .Chart.AppVersion }}
{{- end }}
