package middleware

import (
	"context"
	"github.com/go-kit/kit/endpoint"
	"github.com/sirupsen/logrus"
	"time"
)

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
					"method":          method,
					"took":            time.Since(begin),
					"transport_error": err,
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
