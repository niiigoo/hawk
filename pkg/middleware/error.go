package middleware

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-kit/kit/endpoint"
	"github.com/niiigoo/hawk/pkg/exception"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"net/http"
	"runtime/debug"
)

func CatchPanic(method string, next endpoint.Endpoint) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				if e, ok := r.(error); ok {
					err = e
				} else {
					err = errors.New(fmt.Sprintf("%v", r))
				}
				response = nil
				err = exception.ErrorLog(ctx, logrus.FatalLevel, "error.panic", err, nil, http.StatusInternalServerError, codes.Internal, logrus.Fields{
					"method": method,
					"stack":  string(debug.Stack()),
				})
			}
		}()

		return next(ctx, request)
	}
}
