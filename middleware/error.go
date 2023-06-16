package middleware

import (
	"context"
	"github.com/go-kit/kit/endpoint"
	"github.com/sirupsen/logrus"
	"runtime/debug"
)

func CatchPanic(method string, next endpoint.Endpoint) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = r.(error)
				log := GetLogger(ctx)
				log.WithError(err).WithFields(logrus.Fields{
					"method": method,
					"stack":  string(debug.Stack()),
				}).Error("panic recovered")
			}
		}()

		return next(ctx, request)
	}
}
