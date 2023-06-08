
{{ with $te := .}}
    {{range $i := .Methods}}
		{{ if $i.RequestStream }}
            func (s {{ToLower $te.ServiceName}}Service) {{.Name}}(stream pb.{{GoName $te.ServiceName}}_{{GoName .Name}}Server) error {
                return nil
            }
		{{ else if $i.ResponseStream }}
            func (s {{ToLower $te.ServiceName}}Service) {{.Name}}(in *pb.{{GoName .Request}}, stream pb.{{GoName $te.ServiceName}}_{{GoName .Name}}Server) error {
                return nil
            }
		{{ else }}
            func (s {{ToLower $te.ServiceName}}Service) {{.Name}}(ctx context.Context, in *pb.{{GoName .Request}}) (*pb.{{GoName .Response}}, error){
                var resp pb.{{GoName .Response}}
                return &resp, nil
            }
		{{ end }}
    {{end}}
{{- end}}
