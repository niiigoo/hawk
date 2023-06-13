package kit

import (
	"github.com/iancoleman/strcase"
	"github.com/niiigoo/hawk/kit/generic"
	"github.com/niiigoo/hawk/kit/handlers"
	tplFiles "github.com/niiigoo/hawk/kit/template"
	"github.com/niiigoo/hawk/proto"
	"github.com/pkg/errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Generator interface {
	Init(args ...string) error
	Service(file ...string) error
}

type generator struct {
	protoService proto.Parser
	repo         Repository
	dir          string
}

func NewGenerator() Generator {
	dir, _ := os.Getwd()
	return &generator{
		protoService: proto.NewService(),
		repo:         NewRepository(),
		dir:          dir,
	}
}

func (g generator) Init(args ...string) error {
	var pkg, name string
	if len(args) > 0 {
		pkg = args[0]
		if len(args) > 1 {
			name = args[1]
		} else {
			name = filepath.Base(pkg)
		}
	} else {
		pkg = filepath.Base(g.dir)
		name = pkg
	}

	err := g.repo.GoModInit(pkg)
	if err != nil {
		return errors.Wrap(err, "failed to initialize go mod")
	}

	name = strcase.ToLowerCamel(name)
	err = g.protoService.CreateFile(name+".proto", name, strcase.ToCamel(name))
	if err != nil {
		return err
	}

	return nil
}

func (g generator) Service(args ...string) error {
	err := g.downloadDependencies()
	if err != nil {
		return errors.Wrap(err, "failed to download dependencies")
	}

	f, err := g.protoService.DetectFile(args...)
	if err != nil {
		return errors.Wrap(err, "proto file not found")
	}

	err = g.protoService.Parse(f)
	if err != nil {
		return errors.Wrapf(err, "failed to parse proto file '%s'", f)
	}

	err = g.protoService.CompileProto(f, g.dir, g.dir, "$GOPATH/src/github.com/googleapis/googleapis")
	if err != nil {
		return errors.Wrap(err, "protoc failed")
	}

	module, err := g.repo.GetGoModule(g.dir)
	if err != nil {
		return errors.Wrap(err, "failed to read file 'go.mod'")
	}

	prevFiles, err := g.repo.OpenFiles(g.dir, filepath.Base(g.dir), "handlers")
	if err != nil {
		return err
	}

	config := generic.Config{
		GoPackage:     module,
		PBPackage:     module,
		Version:       "",
		VersionDate:   "",
		PreviousFiles: prevFiles,
	}
	files, err := g.generateGoKit(config)
	if err != nil {
		return errors.Wrap(err, "failed to generate service files")
	}

	for name, content := range files {
		err = g.repo.WriteFile("./"+name, content)
		if err != nil {
			return errors.Wrapf(err, "failed to write file '%s'", name)
		}
	}

	err = g.repo.GoModTidy()
	if err != nil {
		return errors.Wrap(err, "`go mod tidy` failed")
	}

	return nil
}

func (g generator) downloadDependencies() error {
	return g.repo.GitClone(
		os.Getenv("GOPATH")+"/src/",
		"github.com/googleapis/googleapis",
	)
}

// generateGoKit returns a go-kit service generated from a service definition,
// the package to the root of the generated service goPackage, the package
// to the .pb.go service struct files (goPBPackage) and any previously generated files.
func (g generator) generateGoKit(conf generic.Config) (map[string]io.Reader, error) {
	if len(g.protoService.Definition().Services) == 0 {
		return nil, errors.New("no service found")
	}
	svc := g.protoService.Definition().Services[0]

	codeGenFiles := make(map[string]io.Reader)
	var err error

	// Remove the suffix "service" since it's added back in by templatePathToActual
	svcName := strings.TrimSuffix(strings.ToLower(svc.Name), "service")
	helper := generic.NewData(svc, conf)
	for _, tpl := range tplFiles.AssetNames() {
		parts := strings.Split(tpl, ".")
		if len(parts) > 3 {
			tpl = parts[0] + "." + strings.Join(parts[2:], ".")
		}

		// Re-derive the actual path for this file based on the service output
		// path provided by the hawk main.go
		actualPath := g.templatePathToActual(tpl, svcName)
		if _, ok := codeGenFiles[actualPath]; ok {
			continue
		}

		var r generic.Renderable
		switch tpl {
		case handlers.ServerHandlerPath:
			r, err = handlers.New(svc, conf.PreviousFiles[actualPath])
			if err != nil {
				return nil, errors.Wrapf(err, "cannot parse previous handler: %q", tpl)
			}
		case handlers.HookPath:
			r = handlers.NewHook(conf.PreviousFiles[actualPath])
		case handlers.MiddlewaresPath:
			m := handlers.NewMiddlewares()
			m.Load(conf.PreviousFiles[actualPath])
			r = m
		}
		file, err := g.repo.GenerateFile(tpl, r, helper)
		if err != nil {
			return nil, errors.Wrap(err, "cannot render template")
		}

		codeGenFiles[actualPath] = file
	}

	return codeGenFiles, nil
}

// templatePathToActual accepts a templateFilePath and the svcName of the
// service and returns what the relative file path of what should be written to
// disk
func (g generator) templatePathToActual(tplPath, svcName string) string {
	// Switch "NAME" in path with svcName.
	// i.e. for svcName = addsvc; /NAME -> /addsvc-service/addsvc
	actual := strings.Replace(tplPath, "NAME", svcName, -1)

	actual = strings.TrimSuffix(actual, ".tpl")

	return actual
}
