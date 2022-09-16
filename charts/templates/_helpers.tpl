{{/* vim: set filetype=mustache: */}}

{{/* Expand the name of the chart.*/}}
{{- define "carina.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "domainName.name" -}}
"{{ .Release.Name }}-controller.{{ .Release.Namespace }}.svc"
{{- end -}}
{{- define "svcDnsName.name" -}}
- "{{ .Release.Name }}-controller"
- "{{ .Release.Name }}-controller.{{ .Release.Namespace }}"
- "{{ .Release.Name }}-controller.{{ .Release.Namespace }}.svc"
{{- end -}}

{{/* labels for helm resources */}}
{{- define "carina.labels" -}}
labels:
  class: carina
  app: csi-carina-controller
  release:  "{{ .Release.Name }}"
  app.kubernetes.io/instance: "{{ .Release.Name }}"
  app.kubernetes.io/managed-by: "{{ .Release.Service }}"
  app.kubernetes.io/name: "{{ template "carina.name" . }}"
  app.kubernetes.io/version: "{{ .Chart.AppVersion }}"
  helm.sh/chart: "{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}"
{{- end -}}

{{/* labels for helm resources */}}
{{- define "carina-node.labels" -}}
labels:
  class: carina
  app: csi-carina-node
  release:  "{{ .Release.Name }}"
  app.kubernetes.io/instance: "{{ .Release.Name }}"
  app.kubernetes.io/managed-by: "{{ .Release.Service }}"
  app.kubernetes.io/name: "{{ template "carina.name" . }}"
  app.kubernetes.io/version: "{{ .Chart.AppVersion }}"
  helm.sh/chart: "{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}"
{{- end -}}

{{/* pull secrets for containers */}}
{{- define "carina.pullSecrets" -}}
{{- if .Values.imagePullSecrets }}
imagePullSecrets:
{{- range .Values.imagePullSecrets }}
  - name: {{ . }}
{{- end }}
{{- end }}
{{- end -}}


{{- define "nodeInitImage" -}}
{{- if hasPrefix "/" .Values.image.nodeInitImage.repository }}
{{- printf "%s%s:%s"  .Values.image.baseRepo .Values.image.nodeInitImage.repository   .Values.image.nodeInitImage.tag  -}}
{{- else }}
{{- printf "%s:%s"   .Values.image.nodeInitImage.repository   .Values.image.nodeInitImage.tag  -}}
{{- end }}
{{- end -}}



{{/* Allow KubeVersion to be overridden. */}}
{{- define "carina.kubeVersion" -}}
  {{- default .Capabilities.KubeVersion.Version .Values.kubeVersionOverride -}}
{{- end -}}

{{/* Get NodeStorageResource API Version */}}
{{- define "carina.nodestorageresources.apiVersion" -}}
  {{- if and (.Capabilities.APIVersions.Has "apiextensions.k8s.io/v1") (semverCompare ">= 1.19-0" (include "carina.kubeVersion" .)) -}}
      {{- print "apiextensions.k8s.io/v1" -}}
  {{- else if .Capabilities.APIVersions.Has "apiextensions.k8s.io/v1beta1" -}}
    {{- print "apiextensions.k8s.io/v1beta1" -}}
  {{- else -}}
    {{- print "apiextensions.k8s.io/v1beta1" -}}
  {{- end -}}
{{- end -}}



