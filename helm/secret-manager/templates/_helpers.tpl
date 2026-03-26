{{/*
Expand the name of the chart.
*/}}
{{- define "secret-manager.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "secret-manager.fullname" -}}
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
{{- define "secret-manager.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "secret-manager.labels" -}}
helm.sh/chart: {{ include "secret-manager.chart" . }}
{{ include "secret-manager.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "secret-manager.selectorLabels" -}}
app.kubernetes.io/name: {{ include "secret-manager.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "secret-manager.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "secret-manager.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Database URL
*/}}
{{- define "secret-manager.databaseUrl" -}}
{{- if .Values.backend.databaseUrl }}
{{- .Values.backend.databaseUrl }}
{{- else if .Values.postgres.enabled }}
{{- printf "postgres://admin:changeme-production-password@postgres:5432/secretmanager?sslmode=disable" }}
{{- else }}
{{- required "Either postgres.enabled must be true or backend.databaseUrl must be set" .Values.backend.databaseUrl }}
{{- end }}
{{- end }}
