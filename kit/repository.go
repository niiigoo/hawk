package kit

import (
	"bytes"
	"fmt"
	"github.com/niiigoo/hawk/kit/generic"
	"github.com/niiigoo/hawk/kit/template"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"go/format"
	"golang.org/x/mod/modfile"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type Repository interface {
	WriteFile(name string, reader io.Reader) error
	OpenFiles(dir string, excludes ...string) (map[string]io.Reader, error)
	GitClone(path, repo string) error
	GoModInit(pkg string) error
	GoModTidy() error
	GetGoModule(dir string) (string, error)
	GenerateFile(tpl string, gen generic.Renderable, data *generic.Data) (io.Reader, error)
}

type repository struct {
}

func NewRepository() Repository {
	return &repository{}
}

func (r repository) GoModInit(pkg string) error {
	return exec.Command("go", "mod", "init", pkg).Run()
}

func (r repository) GoModTidy() error {
	return exec.Command("go", "mod", "tidy").Run()
}

// WriteFile creates or overrides a file with the data from the reader
func (r repository) WriteFile(name string, reader io.Reader) error {
	err := os.MkdirAll(filepath.Dir(name), 0666)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(name, os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return err
	}

	buf := make([]byte, 1024)
	for {
		// read a chunk
		n, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}

		// write a chunk
		if _, err = f.Write(buf[:n]); err != nil {
			return err
		}
	}

	return nil
}

// OpenFiles returns a map[string]io.Reader representing the files in dir
func (r repository) OpenFiles(dir string, nested ...string) (map[string]io.Reader, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, nil
	}

	includes := map[string]bool{}
	for _, n := range nested {
		includes[n] = true
	}

	files := make(map[string]io.Reader)

	addFileToFiles := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if _, ok := includes[info.Name()]; ok {
				return nil
			}
			return filepath.SkipDir
		}

		file, ioErr := os.Open(path)
		if ioErr != nil {
			return errors.Wrapf(ioErr, "cannot read file: %v", path)
		}

		// trim the prefix of the path to the proto files from the full path to the file
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		// ensure relPath is unix-style, so it matches what we look for later
		relPath = filepath.ToSlash(relPath)

		files[relPath] = file

		return nil
	}

	err := filepath.Walk(dir, addFileToFiles)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot fully walk directory %v", dir)
	}

	return files, nil
}

func (r repository) GitClone(path, repo string) error {
	if _, err := os.Stat(filepath.Join(path, repo)); os.IsNotExist(err) {
		println("Downloading dependencies...")
		cmd := exec.Command("git", "clone", "https://"+repo, filepath.Join(path, repo))
		err = r.run(cmd)
		if err != nil {
			return errors.Wrapf(err, "failed to clone '%s'", repo)
		}
	}

	return nil
}

func (r repository) GetGoModule(dir string) (string, error) {
	goModBytes, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}

	return modfile.ModulePath(goModBytes), nil
}

// GenerateFile contains logic to choose how to render a template file
// based on path and if that file was generated previously. It accepts a
// template path to render, a templateExecutor to apply to the template, and a
// map of paths to files for the previous generation. It returns an
// io.Reader representing the generated file.
func (r repository) GenerateFile(tpl string, gen generic.Renderable, data *generic.Data) (io.Reader, error) {
	var reader io.Reader
	var err error
	if gen != nil {
		reader, err = gen.Render(data)
		if err != nil {
			return nil, err
		}
	} else {
		if reader, err = r.applyTemplateFromPath(tpl, data); err != nil {
			return nil, errors.Wrapf(err, "cannot render template: %s", tpl)
		}
	}

	codeBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// ignore error as we want to write the code either way to inspect after writing to disk
	formatted, err := format.Source(codeBytes)
	if err != nil {
		log.WithError(err).WithField("file", tpl).Warn("Code formatting error, generated service will not build, outputting not formatted code")
		return bytes.NewReader(codeBytes), nil
	} else {
		return bytes.NewReader(formatted), nil
	}
}

// applyTemplateFromPath calls applyTemplate with the template
func (r repository) applyTemplateFromPath(tpl string, data *generic.Data) (io.Reader, error) {
	tplBytes, err := template.Asset(tpl)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to find template file: %v", tpl)
	}

	return data.ApplyTemplate(string(tplBytes), tpl)
}

func (r repository) run(cmd *exec.Cmd) error {
	out := new(bytes.Buffer)
	outErr := new(bytes.Buffer)
	cmd.Stdout = out
	cmd.Stderr = outErr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Run cmd: %s\n", cmd.String())
		fmt.Println(out.String())
		fmt.Println(outErr.String())
		if e, ok := err.(*exec.ExitError); ok && len(e.Stderr) > 0 {
			err = errors.Wrap(e, string(e.Stderr))
		}
	}

	return err
}
