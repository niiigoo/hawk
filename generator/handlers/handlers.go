// Package handlers renders the Go source files found in <svcname>/handlers/.
// Most importantly, it handles rendering and modifying the
// <svcname>/handlers/handlers.go file, while making sure that existing code in
// that handlers.go file is not deleted.
package handlers

import (
	"bytes"
	"github.com/niiigoo/hawk/generator/generic"
	"github.com/niiigoo/hawk/generator/template"
	protoParser "github.com/niiigoo/hawk/proto"
	log "github.com/sirupsen/logrus"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"strings"
)

// NewService is an exported func that creates a new service
// it will not be defined in the service definition but is required
const ignoredFunc = "NewService"

// ServerHandlerPath is the relative path to the server handler template file
const ServerHandlerPath = "handlers/handlers.gotemplate"

// ServerHandlerMethodsPath is the relative path to the server handler method template file
const ServerHandlerMethodsPath = "handlers/handlers.methods.gotemplate"

// New returns a hawk.Renderable capable of updating server handlers.
// The previous version of the server handler should be provided.
func New(svc *protoParser.Service, prev io.Reader) (generic.Renderable, error) {
	var h handler
	log.WithField("Service Methods", len(svc.Methods)).Debug("Handler being created")
	h.mMap = newMethodMap(svc.Methods)
	h.service = svc

	if prev == nil {
		return &h, nil
	}

	h.fileSet = token.NewFileSet()
	var err error
	if h.ast, err = parser.ParseFile(h.fileSet, "", prev, parser.ParseComments); err != nil {
		return nil, err
	}

	return &h, nil
}

// methodMap stores all the service methods defined in the service.proto. It
// stores these methods by their string name. In order to not overwrite
// existing methods in the 'handlers/handlers.go' file, methods which already
// exist in the 'handlers/handlers.go' file will be removed from this
// methodMap.
type methodMap map[string]*protoParser.Method

func newMethodMap(meths []*protoParser.Method) methodMap {
	mMap := make(methodMap, len(meths))
	for _, m := range meths {
		mMap[m.Name] = m
	}
	return mMap
}

type handler struct {
	fileSet *token.FileSet
	service *protoParser.Service
	mMap    methodMap
	// The Abstract Syntax Tree (AST) of the existing go code found in
	// 'handlers/handlers.go'. If the 'handlers/handlers.go' file does not
	// exist, then ast will be nil.
	ast *ast.File
}

type handlerData struct {
	ServiceName string
	Methods     []*protoParser.Method
}

// Render returns an io.Reader with the go code of the server handler. That
// server handler ('handlers.go') has functions for all ServiceMethods in the
// service definition.
func (h *handler) Render(data *generic.Data) (io.Reader, error) {
	// implies that there is not an existing 'handlers/handlers.go' file, and we
	// can safely render the default template without worry.
	if h.ast == nil {
		return applyServerTpl(data)
	}

	// Remove exported methods not defined in service definition
	// and remove methods defined in the previous file from methodMap
	log.WithField("Service Methods", len(h.mMap)).Debug("Before prune")
	// Lowercase the service name before pruning because the templates all
	// lowercase the service name when generating code to ensure Identifiers
	// incorporating the service name remain unexported.
	h.ast.Decls = h.mMap.pruneDecls(h.ast.Decls, strings.ToLower(data.Service.Name))
	log.WithField("Service Methods", len(h.mMap)).Debug("After prune")

	// create a new handlerData, and add all methods not defined in the existing 'handlers/handlers.go' file
	ex := handlerData{
		ServiceName: data.Service.Name,
	}

	// If there are no methods to template then exit early
	if len(h.mMap) == 0 {
		return h.buffer()
	}

	for k, v := range h.mMap {
		log.WithField("Method", k).
			Info("Generating handler from rpc definition")
		ex.Methods = append(ex.Methods, v)
	}

	// get the code out of the ast
	code, err := h.buffer()
	if err != nil {
		return nil, err
	}

	// render the server for all methods not already defined
	newCode, err := applyServerMethsTpl(ex)

	if err != nil {
		return nil, err
	}

	if _, err = code.ReadFrom(newCode); err != nil {
		return nil, err
	}

	return code, nil
}

func (h *handler) buffer() (*bytes.Buffer, error) {
	code := bytes.NewBuffer(nil)
	err := printer.Fprint(code, h.fileSet, h.ast)

	if err != nil {
		return nil, err
	}

	return code, nil
}

// pruneDecls constructs a new []ast.Decls with the exported funcs in decls
// who's names are not keys in methodMap and/or does not have the function
// receiver svcName + "Service" ("Handler func")  removed.
//
// When a "Handler func" is not removed from decls that funcs name is also
// deleted from methodMap, resulting in a methodMap only containing keys and
// values for functions defined in the service but not in the handler ast.
//
// In addition, pruneDecls will update unremoved "Handler func"s input
// parameters and output results to by the types described in methodMap's
// serviceMethod for that "Handler func".
func (m methodMap) pruneDecls(decls []ast.Decl, svcName string) []ast.Decl {
	var newDecls []ast.Decl
	for _, d := range decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			name := x.Name.Name
			// Special case NewService and ignore unexported
			if name == ignoredFunc || !ast.IsExported(name) {
				log.WithField("Func", name).
					Debug("Ignoring")
				newDecls = append(newDecls, x)
				continue
			}
			if ok := isValidFunc(x, m, svcName); ok == true {
				updateParams(x, m[name])
				updateResults(x, m[name])
				newDecls = append(newDecls, x)
				delete(m, name)
			}
		default:
			newDecls = append(newDecls, d)
		}

	}
	return newDecls
}

// updateParams updates the second param of f to be `X`.{m.RequestType.Name}.
// For example, this function signature:
//
//	func ProtoMethod(ctx context.Context, *pb.Old)
//
// will become the following kind of function signature, where the old input type is
// replaced by the new input type defined in m.RequestType.Name:
//
//	func ProtoMethod(ctx context.Context, *pb.{m.RequestType.Name})...
func updateParams(f *ast.FuncDecl, m *protoParser.Method) {
	if f.Type.Params.NumFields() != 2 {
		log.WithField("Function", f.Name.Name).
			Warn("Function params signature should be func NAME(ctx context.Context, in *pb.TYPE), cannot fix")
		return
	}
	updatePBFieldType(f.Type.Params.List[1].Type, m.Request)
}

// updateResults updates the first result of f to be `X`.{m.ResponseType.Name}.
// For example, this function signature:
//
//	func ProtoMethod(...) (*pb.Old, error)
//
// will become the following function signature, where the prior return type is
// replaced with the return type defined in m.ResponseType.Name:
//
//	func ProtoMethod(...) (*pb.{m.ResponseType.Name}, error)
func updateResults(f *ast.FuncDecl, m *protoParser.Method) {
	if f.Type.Results.NumFields() != 2 {
		log.WithField("Function", f.Name.Name).
			Warn("Function results signature should be (*pb.TYPE, error), cannot fix")
		return
	}
	updatePBFieldType(f.Type.Results.List[0].Type, m.Response)
}

// updatePBFieldType updates t if in the form X.Sel/*X.Sel to X.newType/*X.newType.
func updatePBFieldType(t ast.Expr, newType string) {
	// *pb.TYPE -> pb.TYPE
	if ptr, _ := t.(*ast.StarExpr); ptr != nil {
		t = ptr.X
	}
	// pb.TYPE -> TYPE
	if sel, _ := t.(*ast.SelectorExpr); sel != nil {
		//pb.SOMETYPE -> pb.newType
		sel.Sel.Name = newType
	}
}

// isValidFunc indicates whether the function declaration here is a function
// declaration which is allowed to exist in handlers/handlers.go. The criteria
// for functions which are allowed in 'handlers/handlers.go' are any of the
// following:
//
//  1. The function is private
//  2. The function is a method of our server struct (e.g. fooStruct) AND
//     it's also a method defined in the generated .pb.go server interface.
//
// These criteria are pretty strict, making many things invalid and thus will
// be removed. Some of the things which are invalid include:
//
//   - Any public function which is not a method of the hawk-created server
//     struct is not valid and will be removed.
//   - Any public method of the hawk-created server which doesn't exist on
//     the .pb.go server interface is not valid and will be removed.
func isValidFunc(f *ast.FuncDecl, m methodMap, svcName string) bool {
	name := f.Name.String()
	if !ast.IsExported(name) {
		log.WithField("Func", name).
			Debug("Unexported function; ignoring")
		return true
	}

	v := m[name]

	if v == nil {
		log.WithField("Method", name).
			Info("Method does not exist in service definition as an rpc; removing")
		return false
	}

	rName := recvTypeToString(f.Recv)
	if rName != svcName+"Service" {
		log.WithField("Func", name).WithField("Receiver", rName).
			Info("Func is exported with improper receiver; removing")
		return false
	}

	log.WithField("Func", name).
		Debug("Method already exists in service definition; ignoring")

	return true
}

// recvTypeToString accepts an *ast.FuncDecl.Recv recv, and returns the
// string of the recv type.
//
//	func (s Foo) Test() {} -> "Foo"
func recvTypeToString(recv *ast.FieldList) string {
	if recv == nil ||
		recv.List[0].Type == nil {
		log.Debug("Function has no reciever")
		return ""
	}

	return exprString(recv.List[0].Type)
}

// exprString returns the string representation of
// ast.Expr for function receivers, parameters, and results.
func exprString(e ast.Expr) string {
	var prefix string
	// *Foo -> Foo
	if ptr, _ := e.(*ast.StarExpr); ptr != nil {
		prefix = "*"
		e = ptr.X
	}
	// *foo.Foo or foo.Foo
	if sel, _ := e.(*ast.SelectorExpr); sel != nil {
		// *foo.Foo -> foo.Foo
		if ptr, _ := e.(*ast.StarExpr); ptr != nil {
			prefix = "*"
			e = ptr.X
		}
		// foo.Foo
		if x, _ := sel.X.(*ast.Ident); x != nil {
			return prefix + x.Name + "." + sel.Sel.Name
		}
		return ""
	}

	// Foo
	if base, _ := e.(*ast.Ident); base != nil {
		return prefix + base.Name
	}

	return ""
}

func applyServerTpl(exec *generic.Data) (io.Reader, error) {
	log.Debug("Rendering handler for the first time")
	tpl, err := template.Asset(ServerHandlerPath)
	if err != nil {
		return nil, err
	}
	return exec.ApplyTemplate(string(tpl), "ServerTpl")
}

func applyServerMethsTpl(exec handlerData) (io.Reader, error) {
	tpl, err := template.Asset(ServerHandlerMethodsPath)
	if err != nil {
		return nil, err
	}
	return generic.ApplyTemplate(string(tpl), "ServerMethsTpl", exec, generic.FuncMap)
}
