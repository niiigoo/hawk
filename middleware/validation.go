package middleware

import (
	"context"
	"github.com/bufbuild/protovalidate-go"
	"github.com/go-kit/kit/endpoint"
	"github.com/niiigoo/hawk/exception"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"net/http"
)

func ProtoValidate(validator *protovalidate.Validator) func(string, endpoint.Endpoint) endpoint.Endpoint {
	return func(method string, endpoint endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			err = validator.Validate(request.(proto.Message))
			if err != nil {
				if _, ok := err.(*protovalidate.ValidationError); ok {
					fields := logrus.Fields{
						"method": method,
					}
					if id := ctx.Value("user"); id != nil {
						fields["user"] = id
					}
					return nil, exception.ErrorLog(ctx, logrus.InfoLevel, nil, "error.validate", exception.ProtoValidationReasons(err), http.StatusUnprocessableEntity, codes.InvalidArgument, fields)
				} else {
					return nil, exception.ErrorLog(ctx, logrus.ErrorLevel, err, "error.validate.exec", nil, http.StatusInternalServerError, codes.Internal, nil)
				}
			}
			return endpoint(ctx, request)
		}
	}
}
