{{/*
Common labels
*/}}
{{- define "microservices.labels" -}}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end }}

{{/*
Service image with optional registry
*/}}
{{- define "microservices.image" -}}
{{- if .global.imageRegistry -}}
{{ .global.imageRegistry }}/{{ .image }}:{{ .tag }}
{{- else -}}
{{ .image }}:{{ .tag }}
{{- end -}}
{{- end }}
