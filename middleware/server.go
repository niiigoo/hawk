package middleware

import (
	"context"
	"github.com/go-kit/kit/endpoint"
	kitHttp "github.com/go-kit/kit/transport/http"
	"github.com/sirupsen/logrus"
	"net/http"
)

var defaultLogger *logrus.Entry

// RequestBearerFromCookie
// Add the value of a cookie (if present) as bearer token to the context using the key `Authorization`
func RequestBearerFromCookie(cookie string) kitHttp.ServerOption {
	return kitHttp.ServerBefore(requestBearerFromCookie(cookie))
}

func requestBearerFromCookie(cookie string) kitHttp.RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		c, err := r.Cookie(cookie)
		if err == nil {
			ctx = context.WithValue(ctx, "authorization", "bearer "+c.Value)
			ctx = context.WithValue(ctx, "Authorization", "bearer "+c.Value)
		}
		return ctx
	}
}

// CookieToContext
// Add the value of a cookie (if present) to the context using the provided name as key
func CookieToContext(cookie, name string) kitHttp.ServerOption {
	return kitHttp.ServerBefore(func(ctx context.Context, r *http.Request) context.Context {
		c, err := r.Cookie(cookie)
		if err == nil {
			ctx = context.WithValue(ctx, name, c.Value)
		}
		return ctx
	})
}

// LoggerToContextHTTP
// Adds a log entry to the context with HTTP request related default fields
func LoggerToContextHTTP(logger *logrus.Entry, fields func(r *http.Request) logrus.Fields) kitHttp.ServerOption {
	return kitHttp.ServerBefore(func(ctx context.Context, r *http.Request) context.Context {
		if logger == nil {
			return ctx
		}
		defaultLogger = logger

		f := fields(r)
		if len(f) == 0 {
			ctx = context.WithValue(ctx, "log", logger)
		} else {
			ctx = context.WithValue(ctx, "log", logger.WithFields(f))
		}

		return ctx
	})
}

// LoggerToContext
// Adds a log entry to the context with request related default fields
func LoggerToContext(logger *logrus.Entry) func(string, endpoint.Endpoint) endpoint.Endpoint {
	return func(method string, next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			if logger != nil {
				defaultLogger = logger
				log := GetLogger(ctx)
				if log == nil {
					log = logger
				}

				log = log.WithField("method", method)
				if t, ok := ctx.Value("transport").(string); ok {
					log = log.WithField("transport", t)
				}

				ctx = context.WithValue(ctx, "log", log)
			}

			return next(ctx, request)
		}
	}
}

// GetLogger
// Returns the log entry from the context
func GetLogger(ctx context.Context) *logrus.Entry {
	if ctx == nil {
		return defaultLogger
	}

	if l, ok := ctx.Value("log").(*logrus.Entry); ok {
		return l
	}

	return defaultLogger
}
