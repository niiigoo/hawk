package exception

import (
	"context"
	"encoding/json"
	"github.com/bufbuild/protovalidate-go"
	kit "github.com/go-kit/kit/transport/http"
	"github.com/google/uuid"
	"github.com/niiigoo/hawk/middleware"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
)

type FuncLog func(ctx context.Context, msg string, err error, reasons map[string]string, details logrus.Fields) error
type Func func(msg string, reasons map[string]string) error

var (
	Internal        = NewLog(logrus.ErrorLevel, http.StatusInternalServerError, codes.Internal)
	NotFound        = NewLog(logrus.InfoLevel, http.StatusNotFound, codes.NotFound)
	Invalid         = NewLog(logrus.InfoLevel, http.StatusUnprocessableEntity, codes.InvalidArgument)
	Conflict        = NewLog(logrus.InfoLevel, http.StatusConflict, codes.AlreadyExists)
	Unauthenticated = NewLog(logrus.InfoLevel, http.StatusUnauthorized, codes.Unauthenticated)
	AccessDenied    = NewLog(logrus.InfoLevel, http.StatusForbidden, codes.PermissionDenied)
)

type exception struct {
	Message    string            `json:"message"`
	Reasons    map[string]string `json:"reasons,omitempty"`
	ErrorId    string            `json:"id"`
	httpStatus int
	grpcCode   codes.Code
}

type Exception interface {
	error
	kit.StatusCoder
	json.Marshaler
	GRPCStatus() *status.Status
}

// NewLog returns an error function containing the message and status codes. Errors are logged.
// The actual error and reasons can be passed later.
func NewLog(logLevel logrus.Level, httpStatus int, grpcCode codes.Code) FuncLog {
	return func(ctx context.Context, msg string, err error, reasons map[string]string, details logrus.Fields) error {
		errId := uuid.New().String()

		if log := middleware.GetLogger(ctx); log != nil {
			if err != nil {
				log = log.WithError(err)
			}
			if details != nil {
				log = log.WithFields(details)
			}

			log.WithFields(logrus.Fields{
				"grpcCode":   grpcCode,
				"statusCode": httpStatus,
				"errorId":    errId,
			}).Log(logLevel, msg)
		}

		return exception{
			Message:    msg,
			Reasons:    reasons,
			ErrorId:    errId,
			httpStatus: httpStatus,
			grpcCode:   grpcCode,
		}
	}
}

// New returns an error function containing the message and status codes.
// The actual error and reasons can be passed later.
func New(httpStatus int, grpcCode codes.Code) Func {
	return func(msg string, reasons map[string]string) error {
		errId := uuid.New().String()

		return exception{
			Message:    msg,
			Reasons:    reasons,
			ErrorId:    errId,
			httpStatus: httpStatus,
			grpcCode:   grpcCode,
		}
	}
}

// ErrorLog returns an error containing the status codes and logs the error
func ErrorLog(ctx context.Context, logLevel logrus.Level, err error, msg string, reasons map[string]string, httpStatus int, grpcCode codes.Code, details logrus.Fields) error {
	return NewLog(logLevel, httpStatus, grpcCode)(ctx, msg, err, reasons, details)
}

// Error returns an error containing the status codes
func Error(msg string, reasons map[string]string, httpStatus int, grpcCode codes.Code) error {
	return New(httpStatus, grpcCode)(msg, reasons)
}

func ProtoValidationReasons(err error) map[string]string {
	reasons := make(map[string]string)

	if e, ok := err.(*protovalidate.ValidationError); ok {
		for _, v := range e.Violations {
			reasons[v.FieldPath] = v.Message
		}
	}

	return reasons
}

func (e exception) Error() string {
	return e.Message
}

// MarshalJSON marshals the error as JSON
func (e exception) MarshalJSON() ([]byte, error) {
	errJson := map[string]interface{}{
		"message":  e.Message,
		"error_id": e.ErrorId,
	}

	if e.Reasons != nil && len(e.Reasons) > 0 {
		errJson["reasons"] = e.Reasons
	}

	return json.Marshal(map[string]interface{}{
		"error": errJson,
	})
}

// StatusCode returns the related HTTP status code
func (e exception) StatusCode() int {
	return e.httpStatus
}

// GRPCStatus returns the related gRPC status code
func (e exception) GRPCStatus() *status.Status {
	return status.New(e.grpcCode, e.ErrorId)
}
