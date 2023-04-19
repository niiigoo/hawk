// Package http provides functions and template helpers for templating
// the http-transport of a go-kit based service.
package http

import (
	"bytes"
	"fmt"
	"github.com/iancoleman/strcase"
	"github.com/niiigoo/hawk/generator/http/templates"
	"github.com/niiigoo/hawk/proto"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"go/format"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"regexp"
	"strconv"
	"strings"
	"text/template"
)

// Helper is the base struct for the data structure containing all the
// information necessary to correctly template the HTTP transport functionality
// of a service. Helper must be built from a Svcdef.
type Helper struct {
	Service        *proto.Service
	Methods        []*Method
	ServerTemplate func(interface{}) (string, error)
	ClientTemplate func(interface{}) (string, error)
}

// NewHelper builds a helper struct from a service declaration. The other
// "New*" functions in this file are there to make this function smaller and
// more testable.
func NewHelper(svc *proto.Service) *Helper {
	// The HTTPAssistFuncs global is a group of function literals defined
	// within templates.go
	rv := Helper{
		Service:        svc,
		Methods:        make([]*Method, 0),
		ServerTemplate: GenServerTemplate,
		ClientTemplate: GenClientTemplate,
	}
	for _, method := range svc.Methods {
		if len(method.HttpBindings) > 0 {
			rv.Methods = append(rv.Methods, NewMethod(method))
		}
	}
	return &rv
}

// NewMethod builds a Method struct from a svcdef.ServiceMethod.
func NewMethod(meth *proto.Method) *Method {
	nMeth := Method{
		Name:         meth.Name,
		RequestType:  meth.Request,
		ResponseType: meth.Response,
	}
	for i := range meth.HttpBindings {
		nBinding := NewBinding(i, meth)
		nBinding.Parent = &nMeth
		nMeth.Bindings = append(nMeth.Bindings, nBinding)
	}
	return &nMeth
}

// NewBinding creates a Binding struct based on a proto.OptionHttp. Because
// NewBinding requires access to some of its parent method's fields, instead
// of passing a proto.OptionHttp directly, you instead pass a
// proto.Method and the index of the HTTPBinding within that methods
// "HTTPBinding" slice.
func NewBinding(i int, meth *proto.Method) *Binding {
	binding := meth.HttpBindings[i]
	nBinding := Binding{
		Label:        meth.Name + EnglishNumber(i),
		PathTemplate: binding.GorillaMuxPath(),
		BasePath:     basePath(binding.PathRaw),
		Method:       binding.Method,
	}
	// Handle oneOfs which need to be specially formed for query params
	for _, param := range binding.Params {
		// only processing oneOf fields
		if param.Type != proto.TypeOneOf {
			continue
		}
		oneOfField := OneofField{
			Name:     strcase.ToCamel(param.Name),
			Location: "query",
		}
		for _, oneofType := range param.OneOfFields {
			option := Field{
				Name: oneofType.Name,
				//QueryParamName: oneOfType.PBFieldName,
				QueryParamName: oneofType.Name,
				CamelName:      strcase.ToCamel(oneofType.Name),
				LowCamelName:   strcase.ToLowerCamel(oneofType.Name),
				Repeated:       oneofType.Repeated,
				IsOptional:     oneofType.Optional,
				//GoType:         oneofType.Type,
				LocalName: fmt.Sprintf("%s%s", strcase.ToCamel(oneofType.Name), strcase.ToCamel(meth.Name)),
			}
			if oneofType.Type == proto.TypeScalar {
				option.GoType = oneofType.Field.Type.Scalar.GoString()
				option.IsBaseType = true
			} else if oneofType.Field.Type.Reference != "" {
				option.GoType = "pb." + oneofType.Field.Type.Reference
			}

			// Modify GoType to reflect pointer or repeated status
			if oneofType.Optional && oneofType.Repeated {
				option.GoType = "[]*" + option.GoType
			} else if oneofType.Repeated {
				option.GoType = "[]" + option.GoType
			}

			option.IsEnum = oneofType.Type == proto.TypeEnum
			option.ConvertFunc, option.ConvertFuncNeedsErrorCheck = createDecodeConvertFunc(option)
			option.TypeConversion = fmt.Sprintf("&pb.%s_%s{%s: %s}", strcase.ToCamel(meth.Request), strcase.ToCamel(oneofType.Name), strcase.ToCamel(oneofType.Name), createDecodeTypeConversion(option))
			option.ZeroValue = getZeroValue(option)

			oneOfField.Options = append(oneOfField.Options, option)
		}
		nBinding.OneOfFields = append(nBinding.OneOfFields, &oneOfField)
	}
	for _, param := range binding.Params {
		// The 'Field' attr of each HTTPParameter always point to it's bound
		// Methods RequestType
		//field := param.Field
		// If the field is a oneof ignore; we handled above already
		if param.Type == proto.TypeOneOf {
			continue
		}
		newField := Field{
			Name:           param.Name,
			QueryParamName: param.Name,
			CamelName:      strcase.ToCamel(param.Name),
			LowCamelName:   strcase.ToLowerCamel(param.Name),
			Location:       string(param.Location),
			Repeated:       param.Repeated,
			IsOptional:     param.Optional,
			LocalName:      fmt.Sprintf("%s%s", strcase.ToCamel(param.Name), strcase.ToCamel(meth.Name)),
		}
		if param.Type == proto.TypeScalar {
			newField.GoType = param.Field.Type.Scalar.GoString()
			newField.IsBaseType = true
		} else if param.Field.Type.Reference != "" {
			newField.GoType = "pb." + param.Field.Type.Reference
		}

		// Modify GoType to reflect pointer or repeated status
		if param.Optional && param.Repeated {
			newField.GoType = "[]*" + newField.GoType
		} else if param.Repeated {
			newField.GoType = "[]" + newField.GoType
		}

		// IsEnum needed for ConvertFunc and TypeConversion logic just below
		newField.IsEnum = param.Type == proto.TypeEnum
		newField.ConvertFunc, newField.ConvertFuncNeedsErrorCheck = createDecodeConvertFunc(newField)
		newField.TypeConversion = createDecodeTypeConversion(newField)

		nBinding.Fields = append(nBinding.Fields, &newField)

		// Enums are allowed in query/path parameters, skip warning
		if newField.IsEnum {
			continue
		}

		// Emit warnings for certain cases
		if !newField.IsBaseType && newField.Location != "body" {
			log.Warnf(
				"%s.%s is a non-base type specified to be located outside of "+
					"the body. Non-base types outside the body may result in "+
					"generated code which fails to compile.",
				meth.Name,
				newField.Name)
		}
		if newField.Repeated && newField.Location == "path" {
			log.Warnf(
				"%s.%s is a repeated field specified to be in the path. "+
					"Repeated fields are not supported in the path and may"+
					"result in generated code which fails to compile.",
				meth.Name,
				newField.Name)
		}
	}
	return &nBinding
}

func GenServerTemplate(exec interface{}) (string, error) {
	code, err := ApplyTemplate("ServerTemplate", templates.ServerTemplate, exec, TemplateFuncs)
	if err != nil {
		return "", err
	}
	encodeFuncSource, err := FuncSourceCode(encodePathParams)
	if err != nil {
		return "", err
	}
	code = FormatCode(code + encodeFuncSource)
	return code, nil
}

func GenClientTemplate(exec interface{}) (string, error) {
	code, err := ApplyTemplate("ClientTemplate", templates.ClientTemplate, exec, TemplateFuncs)
	if err != nil {
		return "", err
	}
	code = FormatCode(code)
	return code, nil
}

// GenServerDecode returns the generated code for the server-side decoding of
// an http request into its request struct.
func (b *Binding) GenServerDecode() (string, error) {
	code, err := ApplyTemplate("ServerDecodeTemplate", templates.ServerDecodeTemplate, b, TemplateFuncs)
	if err != nil {
		return "", err
	}
	code = FormatCode(code)
	return code, nil
}

// GenClientEncode returns the generated code for the client-side encoding of
// that clients request struct into the correctly formatted http request.
func (b *Binding) GenClientEncode() (string, error) {
	code, err := ApplyTemplate("ClientEncodeTemplate", templates.ClientEncodeTemplate, b, TemplateFuncs)
	if err != nil {
		return "", err
	}
	code = FormatCode(code)
	return code, nil
}

// PathSections returns a slice of strings for templating the creation of a
// fully assembled URL with the correct fields in the correct locations.
//
// For example, let's say there's a method "Sum" which accepts a "SumRequest",
// and SumRequest has two fields, 'a' and 'b'. Additionally, lets say that this
// binding for "Sum" has a path of "/sum/{a}". If we call the PathSection()
// method on this binding, it will return a slice that looks like the
// following slice literal:
//
//	[]string{
//	    "\"\"",
//	    "\"sum\"",
//	    "fmt.Sprint(req.A)",
//	}
func (b *Binding) PathSections() []string {
	path := b.PathTemplate
	re := regexp.MustCompile(`{.+:.+}`)
	path = re.ReplaceAllStringFunc(path, func(v string) string {
		return strings.Split(v, ":")[0] + "}"
	})

	isEnum := make(map[string]struct{})
	for _, v := range b.Fields {
		if v.IsEnum {
			isEnum[v.CamelName] = struct{}{}
		}
	}

	var rv []string
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if len(part) > 2 && part[0] == '{' && part[len(part)-1] == '}' {
			name := part[1 : len(part)-1]
			parts := strings.Split(name, ".")
			for idx, part := range parts {
				parts[idx] = strcase.ToCamel(part)
			}
			camelName := strings.Join(parts, ".")

			if _, ok := isEnum[camelName]; ok {
				convert := fmt.Sprintf("fmt.Sprintf(\"%%d\", req.%v)", camelName)
				rv = append(rv, convert)
				continue
			}
			convert := fmt.Sprintf("fmt.Sprint(req.%v)", camelName)
			rv = append(rv, convert)
		} else {
			// Add quotes around things which will be embedded as string literals,
			// so that the 'fmt.Sprint' lines will be unquoted and thus
			// evaluated as code.
			rv = append(rv, `"`+part+`"`)
		}
	}
	return rv
}

// GenQueryUnmarshaler returns the generated code for server-side unmarshaling
// of a query parameter into it's correct field on the request struct.
func (f *Field) GenQueryUnmarshaler() (string, error) {
	queryParamLogic := `
if {{.LocalName}}StrArr, ok := {{.Location}}Params["{{.QueryParamName}}"]; ok {
{{.LocalName}}Str := {{.LocalName}}StrArr[0]`

	pathParamLogic := `
{{.LocalName}}Str := {{.Location}}Params["{{.QueryParamName}}"]`

	genericLogic := `
{{.ConvertFunc}}{{if .ConvertFuncNeedsErrorCheck}}
if err != nil {
	return nil, errors.Wrap(err, fmt.Sprintf("Error while extracting {{.LocalName}} from {{.Location}}, {{.Location}}Params: %v", {{.Location}}Params))
}{{end}}
{{if or .Repeated .IsBaseType .IsEnum}}req.{{.CamelName}} = {{if .IsOptional}}&{{end}}{{.TypeConversion}}{{end}}
`
	mergedLogic := queryParamLogic + genericLogic + "}"
	if f.Location == "path" {
		mergedLogic = pathParamLogic + genericLogic
	}

	code, err := ApplyTemplate("FieldEncodeLogic", mergedLogic, f, TemplateFuncs)
	if err != nil {
		return "", err
	}
	code = FormatCode(code)
	return code, nil
}

// GenQueryUnmarshaler returns the generated code for server-side unmarshaling
// of a query parameter into it's correct field on the request struct.
func (f *OneofField) GenQueryUnmarshaler() (string, error) {
	oneofEnclosure := `
{{- with $oneof := . -}}
	// {{.Name}} oneof
	{{.Name}}CountSet := 0
	{{range $option := $oneof.Options}}

		var {{$option.LocalName}}Str string
		{{$option.LocalName}}StrArr, {{$option.LocalName}}OK := {{$oneof.Location}}Params["{{$option.QueryParamName}}"]
		if {{$option.LocalName}}OK {
			{{$option.LocalName}}Str = {{$option.LocalName}}StrArr[0]
			{{$oneof.Name}}CountSet++
		}
	{{end}}

if {{.Name}}CountSet > 1 {
	return nil, errors.Errorf("only one of ({{range $option := $oneof.Options}}\"{{$option.QueryParamName}}\",{{end}}) allowed")
}

switch {
{{range $option := $oneof.Options}}
case {{$option.LocalName}}OK:
	{{$option.ConvertFunc}}{{if $option.ConvertFuncNeedsErrorCheck}}
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("Error while extracting {{.LocalName}} from {{$option.Location}}, {{$oneof.Location}}Params: %v", {{$oneof.Location}}Params))
	}{{end}}
	req.{{$oneof.Name}} = {{$option.TypeConversion}}
{{end}}
}
{{- end -}}
`

	code, err := ApplyTemplate("FieldEncodeLogic", oneofEnclosure, f, TemplateFuncs)
	if err != nil {
		return "", err
	}
	code = FormatCode(code)
	return code, nil
}

// createDecodeConvertFunc creates a go string representing the function to
// convert the string form of the field to it's correct go type.
func createDecodeConvertFunc(f Field) (string, bool) {
	needsErrorCheck := true
	// We may leverage the below convert logic for repeated base types. By
	// trimming the slice prefix we can easily store the template for the
	// type if needed.
	goType := strings.TrimPrefix(f.GoType, "[]")
	fType := ""
	switch goType {
	case "uint32":
		fType = "%s, err := strconv.ParseUint(%s, 10, 32)"
	case "uint64":
		fType = "%s, err := strconv.ParseUint(%s, 10, 64)"
	case "int32":
		fType = "%s, err := strconv.ParseInt(%s, 10, 32)"
	case "int64":
		fType = "%s, err := strconv.ParseInt(%s, 10, 64)"
	case "bool":
		fType = "%s, err := strconv.ParseBool(%s)"
	case "float32":
		fType = "%s, err := strconv.ParseFloat(%s, 32)"
	case "double":
		fallthrough
	case "float64":
		fType = "%s, err := strconv.ParseFloat(%s, 64)"
	case "string":
		fType = "%s := %s"
		needsErrorCheck = false
	}

	if f.IsEnum && !f.Repeated {
		fType = "%s, err := strconv.ParseInt(%s, 10, 32)"
		return fmt.Sprintf(fType, f.LocalName, f.LocalName+"Str"), true
	}

	// Use json unmarshalling for any custom/repeated messages
	if !f.IsBaseType || f.Repeated {
		// Args representing single custom message types are represented as
		// pointers. To do a bare assignment to a pointer, our rvalue must be a
		// pointer as well. So we special case args of a single custom message
		// type so that the variable LocalName is declared as a pointer.
		singleCustomTypeUnmarshalTmpl := `
err = json.Unmarshal([]byte({{.LocalName}}Str), req.{{.CamelName}})`

		errorCheckingTmpl := `
if err != nil {
	return nil, errors.Wrapf(err, "couldn't decode {{.LocalName}} from %v", {{.LocalName}}Str)
}`
		// All repeated args of any type are represented as slices, and bare
		// assignments to a slice accept a slice as the rvalue. As a result,
		// LocalName will be declared as a slice, and json.Unmarshal handles
		// everything else for us. Addititionally, if a type is a Base type and
		// is repeated, we first attempt to unmarshal the string we're
		// provided, and if that fails, we try to unmarshal the string
		// surrounded by square brackets. If THAT fails, then the string does
		// not represent a valid JSON string and an error is returned.

		repeatedFieldType := strings.TrimPrefix(f.GoType, "[]")
		convertedVar := "converted"
		switch repeatedFieldType {
		case "uint32", "int32", "float32":
			convertedVar = repeatedFieldType + "(converted)"
		}
		repeatedUnmarshalTmpl := `
var {{.LocalName}} {{.GoType}}
{{- if and (.IsBaseType) (not (Contains .GoType "[]byte"))}}
if len({{.LocalName}}StrArr) > 1 {
	{{- if (Contains .GoType "[]string")}}
	{{.LocalName}} = {{.LocalName}}StrArr
	{{- else}}
	{{.LocalName}} = make({{.GoType}}, len({{.LocalName}}StrArr))
	for i, v := range {{.LocalName}}StrArr {
	` + fmt.Sprintf(fType, "converted", "v") + errorCheckingTmpl + `
		{{.LocalName}}[i] = ` + convertedVar + `
	}
	{{- end}}
} else {
{{- end}}
	{{- if (Contains .GoType "[]string")}}
		{{.LocalName}} = strings.Split({{.LocalName}}Str, ",")
	{{- else if and (and .IsBaseType .Repeated) (not (Contains .GoType "[]byte"))}}
	err = json.Unmarshal([]byte({{.LocalName}}Str), &{{.LocalName}})
	if err != nil {
		{{.LocalName}}Str = "[" + {{.LocalName}}Str + "]"
	}
	err = json.Unmarshal([]byte({{.LocalName}}Str), &{{.LocalName}})
	{{- else}}
	err = json.Unmarshal([]byte({{.LocalName}}Str), &{{.LocalName}})
	{{- end}}
{{- if and (.IsBaseType) (not (Contains .GoType "[]byte"))}}
}
{{- end}}`

		var jsonConvTmpl string
		if !f.Repeated {
			jsonConvTmpl = singleCustomTypeUnmarshalTmpl + errorCheckingTmpl
		} else {
			jsonConvTmpl = repeatedUnmarshalTmpl
			if repeatedFieldType != "string" {
				jsonConvTmpl = repeatedUnmarshalTmpl + errorCheckingTmpl
			}
		}
		code, err := ApplyTemplate("UnmarshalNonBaseType", jsonConvTmpl, f, TemplateFuncs)
		if err != nil {
			panic(fmt.Sprintf("Couldn't apply template: %v", err))
		}
		return code, false
	}
	return fmt.Sprintf(fType, f.LocalName, f.LocalName+"Str"), needsErrorCheck
}

// createDecodeTypeConversion creates a go string that converts a 64 bit type
// to a 32 bit type as strconv.ParseInt, ParseUInt, and ParseFloat always
// return the 64 bit type. If the type is not a 64-bit integer type or is
// repeated, then returns the LocalName of that Field.
func createDecodeTypeConversion(f Field) string {
	if f.Repeated {
		// Equivalent of the 'default' case below, but taken early for repeated
		// types.
		return f.LocalName
	}
	fType := ""
	switch f.GoType {
	case "uint32", "int32", "float32":
		fType = f.GoType + "(%s)"
	default:
		fType = "%s"
	}
	if f.IsEnum {
		fType = f.GoType + "(%s)"
	}
	return fmt.Sprintf(fType, f.LocalName)
}

func getZeroValue(f Field) string {
	if !f.IsBaseType || f.Repeated {
		return "nil"
	}
	switch f.GoType {
	case "bool":
		return "false"
	case "string":
		return "\"\""
	default:
		return "0"
	}
}

// getMuxPathTemplate translates gRPC Transcoding path into gorilla/mux
// compatible path template.
func getMuxPathTemplate(path string) string {
	re := regexp.MustCompile(`{.+=.+}`)
	stars := regexp.MustCompile(`\*{2,}`)
	return re.ReplaceAllStringFunc(path, func(v string) string {
		v = strings.Replace(v, "=", ":", 1)
		v = stars.ReplaceAllLiteralString(v, `.+`)
		v = strings.ReplaceAll(v, "*", `[^/]+`)
		return v
	})
}

// The 'basePath' of a path is the section from the start of the string till
// the first '{' character.
func basePath(path string) string {
	parts := strings.Split(path, "{")
	return parts[0]
}

// DigitEnglish is a map of runes of digits zero to nine to their lowercase
// english language spellings.
var DigitEnglish = map[rune]string{
	'0': "Zero",
	'1': "One",
	'2': "Two",
	'3': "Three",
	'4': "Four",
	'5': "Five",
	'6': "Six",
	'7': "Seven",
	'8': "Eight",
	'9': "Nine",
}

// EnglishNumber takes an integer and returns the english words that represents
// that number, in base ten. Examples:
//
//	1  -> "One"
//	5  -> "Five"
//	10 -> "OneZero"
//	48 -> "FourEight"
func EnglishNumber(i int) string {
	n := strconv.Itoa(i)
	rv := ""
	for _, c := range n {
		if e, ok := DigitEnglish[c]; ok {
			rv += e
		}
	}
	return rv
}

// TemplateFuncs contains a series of utility functions to be passed into
// templates and used within those templates.
var TemplateFuncs = template.FuncMap{
	"ToLower":  strings.ToLower,
	"ToUpper":  strings.ToUpper,
	"Title":    cases.Title(language.English).String,
	"GoName":   strcase.ToCamel,
	"Contains": strings.Contains,
	"Label":    EnglishNumber,
	"BasePath": basePath,
}

// ApplyTemplate applies a template with a given name, executor context, and
// function map. Returns the output of the template on success, returns an
// error if template failed to execute.
func ApplyTemplate(name string, tmpl string, executor interface{}, fncs template.FuncMap) (string, error) {
	codeTemplate := template.Must(template.New(name).Funcs(fncs).Parse(tmpl))

	code := bytes.NewBuffer(nil)
	err := codeTemplate.Execute(code, executor)
	if err != nil {
		return "", errors.Wrapf(err, "attempting to execute template %q", name)
	}
	return code.String(), nil
}

// FormatCode takes a string representing some go code and attempts to format
// that code. If formatting fails, the original source code is returned.
func FormatCode(code string) string {
	formatted, err := format.Source([]byte(code))

	if err != nil {
		// Set formatted to code so at least we get something to examine
		formatted = []byte(code)
	}

	return string(formatted)
}
