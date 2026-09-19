{{- define "calendarrabbit.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "calendarrabbit.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "calendarrabbit.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "calendarrabbit.labels" -}}
app.kubernetes.io/name: {{ include "calendarrabbit.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{- end -}}

{{- define "calendarrabbit.backendFullname" -}}
{{- printf "%s-backend" (include "calendarrabbit.fullname" .) -}}
{{- end -}}

{{- define "calendarrabbit.frontendFullname" -}}
{{- printf "%s-frontend" (include "calendarrabbit.fullname" .) -}}
{{- end -}}

{{- define "calendarrabbit.mysqlFullname" -}}
{{- printf "%s-mysql" (include "calendarrabbit.fullname" .) -}}
{{- end -}}

{{- define "calendarrabbit.backendSecretName" -}}
{{- if .Values.backend.secret.existingSecret -}}
{{- .Values.backend.secret.existingSecret -}}
{{- else -}}
{{- printf "%s-secret" (include "calendarrabbit.backendFullname" .) -}}
{{- end -}}
{{- end -}}

{{- define "calendarrabbit.mysqlSecretName" -}}
{{- if .Values.mysql.secret.existingSecret -}}
{{- .Values.mysql.secret.existingSecret -}}
{{- else -}}
{{- printf "%s-secret" (include "calendarrabbit.mysqlFullname" .) -}}
{{- end -}}
{{- end -}}
