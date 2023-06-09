package handlers

import (
	"context"
	"github.com/sirupsen/logrus"

	pb "{{.PBImportPath -}}"
)

var Logger *logrus.Entry

// NewService returns a naive, stateless implementation of Service.
func NewService() pb.{{GoName .Service.Name}}Server {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	Logger = log.WithField("service", "{{ToLower .Service.Name}}")

	return {{ToLower .Service.Name}}Service{}
}

type {{ToLower .Service.Name}}Service struct{
	pb.Unimplemented{{GoName .Service.Name}}Server
}

{{with $te := . }}
	{{range $i := $te.Service.Methods}}
		{{ if $i.RequestStream }}
            func (s {{ToLower $te.Service.Name}}Service) {{.Name}}(stream pb.{{GoName $te.Service.Name}}_{{GoName .Name}}Server) error {
                return nil
            }
		{{ else if $i.ResponseStream }}
            func (s {{ToLower $te.Service.Name}}Service) {{.Name}}(in *pb.{{GoName .Request}}, stream pb.{{GoName $te.Service.Name}}_{{GoName .Name}}Server) error {
                return nil
            }
		{{ else }}
            func (s {{ToLower $te.Service.Name}}Service) {{.Name}}(ctx context.Context, in *pb.{{GoName .Request}}) (*pb.{{GoName .Response}}, error){
                var resp pb.{{GoName .Response}}
                return &resp, nil
            }
		{{ end }}
	{{end}}
{{- end}}
