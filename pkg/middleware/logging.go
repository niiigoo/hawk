package middleware

import (
	"context"
	"github.com/go-kit/kit/endpoint"
	kitHttp "github.com/go-kit/kit/transport/http"
	"github.com/sirupsen/logrus"
	"net/http"
	"time"
)

var defaultLogger *logrus.Entry

// EndpointLogging returns an endpoint middleware that logs the
// duration of each invocation and the resulting error if any.
func EndpointLogging(logger *logrus.Entry, fields logrus.Fields) func(string, endpoint.Endpoint) endpoint.Endpoint {
	return func(method string, next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			log := GetLogger(ctx)
			if log == nil {
				log = logger
			}
			defer func(begin time.Time) {
				log = log.WithFields(logrus.Fields{
					"method": method,
					"took":   time.Since(begin),
					"error":  err,
				})
				if len(fields) > 0 {
					log = log.WithFields(fields)
				}
				log.Info("request completed")
			}(time.Now())
			return next(ctx, request)
		}
	}
}

// LoggerToContextHTTP adds a log entry to the context with HTTP request related default fields
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

// LoggerToContext adds a log entry to the context with request related default fields
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

// GetLogger returns the log entry from the context
func GetLogger(ctx context.Context) *logrus.Entry {
	if ctx == nil {
		return defaultLogger
	}

	if l, ok := ctx.Value("log").(*logrus.Entry); ok {
		return l
	}

	return defaultLogger
}
