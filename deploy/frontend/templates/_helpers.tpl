{{- define "esg-frontend.labels" -}}
app.kubernetes.io/name: esg-frontend
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
{{- define "esg-frontend.selector" -}}
app.kubernetes.io/name: esg-frontend
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
