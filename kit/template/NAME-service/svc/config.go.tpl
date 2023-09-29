package svc

import (
	"github.com/go-kit/kit/transport/http"
)

// Config contains the required fields for running a server
type Config struct {
	ServiceAddr                string
	DebugAddr                  string
	GenericHTTPResponseEncoder http.EncodeResponseFunc
}
