{{ .Title }}
{{- with .Body }}

{{ . }}
{{- end }}

[comment]: # (Please enter the pull request message.)
[comment]: # (Lines formatted like this will be ignored,)
[comment]: # (and an empty message aborts the pull request.)
[comment]: # (The first line will be used as the title and must not be empty.)
[comment]: # ({{ .BaseOwner }}/{{ .BaseRepo }}: merge into {{ .BaseOwner }}:{{ .BaseBranch }} from {{ .HeadOwner }}:{{ .Branch }})
