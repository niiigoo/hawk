package middleware

import (
	"context"
	"net/http"

	"buf.build/go/protovalidate"
	"github.com/go-kit/kit/endpoint"
	"github.com/niiigoo/hawk/pkg/exception"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
)

func ProtoValidate() func(string, endpoint.Endpoint) endpoint.Endpoint {
	return func(method string, endpoint endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (response interface{}, err error) {
			err = protovalidate.Validate(request.(proto.Message))
			if err != nil {
				var validationError *protovalidate.ValidationError
				if errors.As(err, &validationError) {
					fields := logrus.Fields{
						"method": method,
					}
					if id := ctx.Value("user"); id != nil {
						fields["user"] = id
					}
					return nil, exception.ErrorLog(ctx, logrus.InfoLevel, "error.validate", nil, exception.ProtoValidationReasons(err), http.StatusUnprocessableEntity, codes.InvalidArgument, fields)
				}
			}
			return endpoint(ctx, request)
		}
	}
}
