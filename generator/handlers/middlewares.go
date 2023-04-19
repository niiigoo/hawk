package handlers

import (
	"github.com/niiigoo/hawk/generator/generic"
	"github.com/niiigoo/hawk/generator/template"
	"io"

	"github.com/pkg/errors"
)

// MiddlewaresPath is the path to the middleware gotemplate file.
const MiddlewaresPath = "handlers/middlewares.gotemplate"

// NewMiddlewares returns a Renderable that renders the middlewares.go file.
func NewMiddlewares() *Middlewares {
	var m Middlewares

	return &m
}

// Middlewares satisfies the generic.Renderable interface to render
// middlewares.
type Middlewares struct {
	prev io.Reader
}

// Load loads the previous version of the middleware file.
func (m *Middlewares) Load(prev io.Reader) {
	m.prev = prev
}

// Render creates the middlewares.go file. With no previous version it renders
// the templates, if there was a previous version loaded in, it passes that
// through.
func (m *Middlewares) Render(data *generic.Data) (io.Reader, error) {
	if m.prev != nil {
		return m.prev, nil
	}
	tplBytes, err := template.Asset(MiddlewaresPath)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to find template file: %v", MiddlewaresPath)
	}
	return data.ApplyTemplate(string(tplBytes), "Middlewares")
}
