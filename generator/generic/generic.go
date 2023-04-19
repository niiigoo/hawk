package generic

import (
	"bytes"
	"github.com/iancoleman/strcase"
	"github.com/niiigoo/hawk/generator/http"
	"github.com/niiigoo/hawk/proto"
	"github.com/pkg/errors"
	"io"
	"text/template"
)

type Renderable interface {
	Render(*Data) (io.Reader, error)
}

type Config struct {
	GoPackage   string
	PBPackage   string
	Version     string
	VersionDate string

	PreviousFiles map[string]io.Reader
}

// FuncMap contains a series of utility functions to be passed into
// templates and used within those templates.
var FuncMap = template.FuncMap{
	"ToLower": strcase.ToLowerCamel,
	"GoName":  strcase.ToCamel,
}

// Data is passed to templates as the executing struct; its fields
// and methods are used to modify the template
type Data struct {
	// import path for the directory containing the definition .proto files
	ImportPath string
	// import path for .pb.go files containing service structs
	PBImportPath string
	// PackageName is the name of the package containing the service definition
	PackageName string
	// GRPC/Proto service, with all parameters and return values accessible
	Service *proto.Service
	// A helper struct for generating http transport functionality.
	HTTPHelper *http.Helper
	// Helper functions used within the templates
	FuncMap template.FuncMap

	Version     string
	VersionDate string
}

func NewData(svc *proto.Service, conf Config) *Data {
	return &Data{
		ImportPath:   conf.GoPackage,
		PBImportPath: conf.PBPackage,
		PackageName:  conf.PBPackage,
		Service:      svc,
		HTTPHelper:   http.NewHelper(svc),
		FuncMap:      FuncMap,
		Version:      conf.Version,
		VersionDate:  conf.VersionDate,
	}
}

// ApplyTemplate applies the passed template with the Data
func (e *Data) ApplyTemplate(tpl string, tplName string) (io.Reader, error) {
	return ApplyTemplate(tpl, tplName, e, e.FuncMap)
}

// ApplyTemplate is a helper methods that packages can call to render a
// template with any data and func map
func ApplyTemplate(tpl string, tplName string, data interface{}, funcMap template.FuncMap) (io.Reader, error) {
	codeTemplate, err := template.New(tplName).Funcs(funcMap).Parse(tpl)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create template")
	}

	outputBuffer := bytes.NewBuffer(nil)
	err = codeTemplate.Execute(outputBuffer, data)
	if err != nil {
		return nil, errors.Wrap(err, "template error")
	}

	return outputBuffer, nil
}
