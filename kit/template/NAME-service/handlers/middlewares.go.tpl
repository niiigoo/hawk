package handlers

import (
	"context"
	pb "{{.PBImportPath -}}"
	"{{.ImportPath -}} /svc"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/niiigoo/hawk/middleware"
	"github.com/sirupsen/logrus"
	"net/http"
)

// WrapEndpoints accepts the service's entire collection of endpoints, so that a
// set of middlewares can be wrapped around every middleware (e.g., access
// logging and instrumentation), and others wrapped selectively around some
// endpoints and not others (e.g., endpoints requiring authenticated access).
// Note that the final middleware wrapped will be the outermost middleware
// (i.e. applied first)
func WrapEndpoints(in svc.Endpoints) svc.Endpoints {

	// Pass a middleware you want applied to every endpoint.
	// optionally pass in endpoints by name that you want to be excluded
	// e.g.
	// in.WrapAllExcept(authMiddleware, "Status", "Ping")
	in.WrapAllExcept(middleware.CatchPanic)

	// Pass in a svc.LabeledMiddleware you want applied to every endpoint.
	// These middlewares get passed the endpoints name as their first argument when applied.
	// This can be used to write generic metric gathering middlewares that can
	// report the endpoint name for free.
	// github.com/niiigoo/hawk/middleware/endpoint.go for an example.
	// in.WrapAllLabeledExcept(errorCounter(statsdCounter), "Status", "Ping")

	// How to apply a middleware to a single endpoint.
	// in.ExampleEndpoint = authMiddleware(in.ExampleEndpoint)

    // Some middlewares to improve the logging of requests
    // Use `middleware.GetLogger` to benefit from it
	in.WrapAllLabeledExcept(middleware.EndpointLogging(Logger, nil))
	in.WrapAllLabeledExcept(middleware.LoggerToContext(Logger))
	in.WrapAllWithHttpOptionExcept(middleware.LoggerToContextHTTP(Logger, func(r *http.Request) logrus.Fields {
		fields := logrus.Fields{
			"method": r.Method,
			"url":    r.URL.String(),
		}

		// Example for request-based tracing
		if id := r.Header.Get("X-Request-Id"); id != "" {
			fields["request_id"] = id
		} else {
			fields["request_id"] = uuid.NewString()
		}

		// Example for session-based tracing
		if id := r.Header.Get("X-Session-Id"); id != "" {
			fields["session_id"] = id
		}

		return fields
	}))

	return in
}

func WrapService(in pb.{{.Service.Name}}Server) pb.{{.Service.Name}}Server {
	return in
}

// WebSocketGuard protects the webSocket endpoint. The connection is only upgraded if the guard
// function does not report an error. The returned context is valid for the lifetime of the connection.
// `service` has the type `{{ToLower .Service.Name}}Service` and can be cast.
func WebSocketGuard(ctx context.Context, service pb.{{GoName .Service.Name}}Server, r *http.Request) (context.Context, error) {
	return ctx, nil
}

// WebSocketOriginChecker checks the origin header of the request before upgrading the connection.
// @see github.com/gorilla/websocket for more details
func WebSocketOriginChecker(service pb.{{GoName .Service.Name}}Server, r *http.Request) bool {
	origin := r.Header["Origin"]
	if len(origin) == 0 {
		return true
	}
	u, err := url.Parse(origin[0])
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}
