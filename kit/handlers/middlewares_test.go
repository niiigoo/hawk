package handlers

import (
	"github.com/niiigoo/hawk/kit/generic"
	thelper "github.com/niiigoo/hawk/kit/testHelper"
	"github.com/niiigoo/hawk/proto"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func init() {
	gopath = filepath.SplitList(os.Getenv("GOPATH"))
}

func TestRenderPrevEndpoints(t *testing.T) {
	var wantEndpoints = `
		package middlewares

		import (
			"github.com/go-kit/kit/endpoint"
			"github.com/niiigoo/hawk/test/general-service/svc"
		)

		// WrapEndpoint will be called individually for all endpoints defined in
		// the service. Implement this with the middlewares you want applied to
		// every endpoint.
		func WrapEndpoint(in endpoint.Endpoint) endpoint.Endpoint {
			return in
		}

		// WrapEndpoints takes the service's entire collection of endpoints. This
		// function can be used to apply middlewares selectively to some endpoints,
		// but not others, like protecting some endpoints with authentication.
		func WrapEndpoints(in svc.Endpoints) svc.Endpoints {
			return in
		}

		func BarFoo(err error) bool {
			if err != nil {
				return true
			}
			return false
		}
	`

	_, data, err := generalService()
	if err != nil {
		t.Fatal(err)
	}

	middleware := NewMiddlewares()

	middleware.Load(strings.NewReader(wantEndpoints))

	endpoints, err := middleware.Render(data)
	if err != nil {
		t.Fatal(err)
	}

	endpointsBytes, err := io.ReadAll(endpoints)
	if err != nil {
		t.Fatal(err)
	}

	wantFormatted, endpointFormatted, diff := thelper.DiffGoCode(wantEndpoints, string(endpointsBytes))
	if wantFormatted != endpointFormatted {
		t.Fatalf("Endpoints middleware modified unexpectedly:\n\n%s", diff)
	}
}

func generalService() (*proto.Service, *generic.Data, error) {
	const def = `
		syntax = "proto3";

		// General package
		package general;

		import "github.com/niiigoo/hawk/deftree/googlethirdparty/annotations.proto";

		// RequestMessage is so foo
		message RequestMessage {
			string input = 1;
		}

		// ResponseMessage is so bar
		message ResponseMessage {
			string output = 1;
		}

		// ProtoService is a service
		service ProtoService {
			// ProtoMethod is simple. Like a gopher.
			rpc ProtoMethod (RequestMessage) returns (ResponseMessage) {
				// No {} in path and no body, everything is in the query
				option (google.api.http) = {
					get: "/route"
				};
			}
		}
	`
	p := proto.NewService()
	err := p.ParseString(def)
	if err != nil {
		return nil, nil, err
	}
	conf := generic.Config{
		GoPackage: "github.com/niiigoo/hawk/test/general-service",
		PBPackage: "github.com/niiigoo/hawk/test/general-service",
	}

	data := generic.NewData(p.Definition().Services[0], conf)

	return p.Definition().Services[0], data, nil
}
