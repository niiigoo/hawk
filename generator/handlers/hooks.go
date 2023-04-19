package handlers

import (
	"bytes"
	"github.com/niiigoo/hawk/generator/generic"
	"github.com/niiigoo/hawk/generator/template"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"strings"
)

const HookPath = "handlers/hooks.gotemplate"

const hookPathImports = "handlers/hooks.imports.gotemplate"
const hookPathHandler = "handlers/hooks.handler.gotemplate"
const hookPathConfig = "handlers/hooks.config.gotemplate"

// NewHook returns a new HookRender
func NewHook(prev io.Reader) generic.Renderable {
	return &HookRender{
		prev: prev,
	}
}

type HookRender struct {
	prev io.Reader
}

// Render returns an io.Reader with the contents of
// <svcname>/handlers/hooks.go. If hooks.go does not already exist, then it's
// rendered anew from the template. If hooks.go does exist already, then:
//
//  1. Modify the new code so that it will import
//     "{{.ImportPath}}/svc/server" if it doesn't already.
//  2. Add the InterruptHandler if it doesn't exist already
//  3. Add the SetConfig function if it doesn't exist already
func (h *HookRender) Render(data *generic.Data) (io.Reader, error) {
	if h.prev == nil {
		tpl, err := assetTemplates(hookPathImports, hookPathHandler, hookPathConfig)
		if err != nil {

		}
		return data.ApplyTemplate(tpl, "HooksFullTemplate")
	}
	rawPrev, err := io.ReadAll(h.prev)
	if err != nil {
		return nil, err
	}
	code := bytes.NewBuffer(rawPrev)

	fSet := token.NewFileSet()
	past, err := parser.ParseFile(fSet, "", code, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	err = addServerImportIfNotPresent(past, data)
	if err != nil {
		return nil, err
	}

	var existingFuncs = map[string]bool{}
	for _, d := range past.Decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			name := x.Name.Name
			existingFuncs[name] = true
		}
	}
	code = bytes.NewBuffer(nil)
	err = printer.Fprint(code, fSet, past)
	if err != nil {
		return nil, err
	}

	// Both of these functions need to be in hooks.go in order for the service to start.
	hookFuncs := map[string]string{
		"InterruptHandler": hookPathHandler,
		"SetConfig":        hookPathConfig,
	}

	for name, f := range hookFuncs {
		if _, ok := existingFuncs[name]; !ok {
			tpl, err := assetTemplates(f)
			if err != nil {
				return nil, err
			}
			_, _ = code.ReadFrom(strings.NewReader(tpl))
		}
	}
	return code, nil
}

func assetTemplates(templates ...string) (string, error) {
	var data string
	for _, tpl := range templates {
		raw, err := template.Asset(tpl)
		if err != nil {
			return "", err
		}
		data += string(raw)
	}
	return data, nil
}

// addServerImportIfNotPresent ensures that the hooks.go file imports the
// "{{.ImportPath -}} /svc/server" file since the SetConfig function requires
// that import in order to compile. It does this by mutating the handler file
// provided as parameter hf in place.
func addServerImportIfNotPresent(hf *ast.File, exec *generic.Data) error {
	var imports *ast.GenDecl
	for _, decl := range hf.Decls {
		switch decl.(type) {
		case *ast.GenDecl:
			imports = decl.(*ast.GenDecl)
			break
		}
	}

	targetPathTpl := `"{{.ImportPath -}} /svc"`
	r, err := exec.ApplyTemplate(targetPathTpl, "ServerPathTpl")
	if err != nil {
		return err
	}
	tmp, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	targetPath := string(tmp)

	for _, spec := range imports.Specs {
		switch spec.(type) {
		case *ast.ImportSpec:
			imp := spec.(*ast.ImportSpec)
			if imp.Path.Value == targetPath {
				return nil
			}
		}
	}

	nimp := ast.ImportSpec{
		Doc: &ast.CommentGroup{
			List: []*ast.Comment{
				{
					Text: "// This Service",
				},
			},
		},
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: targetPath,
		},
	}
	imports.Specs = append(imports.Specs, &nimp)
	return nil
}
