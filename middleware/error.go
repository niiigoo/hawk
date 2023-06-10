package middleware

import (
	"context"
	"github.com/go-kit/kit/endpoint"
	"runtime/debug"
)

func CatchPanic(next endpoint.Endpoint) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = r.(error)
				log := GetLogger(ctx)
				log.WithError(err).WithField("stack", string(debug.Stack())).Error("panic recovered")
			}
		}()

		return next(ctx, request)
	}
}
