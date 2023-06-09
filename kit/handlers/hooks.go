package handlers

import (
	"github.com/niiigoo/hawk/kit/generic"
	"github.com/niiigoo/hawk/kit/template"
	"io"
)

const HookPath = "handlers/hooks.go.tpl"

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
// rendered anew from the template.
func (h *HookRender) Render(data *generic.Data) (io.Reader, error) {
	if h.prev != nil {
		return h.prev, nil
	}
	tpl, err := template.Asset(HookPath)
	if err != nil {
		return nil, err
	}
	return data.ApplyTemplate(string(tpl), "HooksFullTemplate")
}
