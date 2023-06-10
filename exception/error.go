package exception

import (
	"context"
	"encoding/json"
	kit "github.com/go-kit/kit/transport/http"
	"github.com/google/uuid"
	"github.com/niiigoo/hawk/middleware"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
)

type Func func(ctx context.Context, reasons map[string]string, details ...string) error

var (
	Internal         = New(logrus.ErrorLevel, "error.internal", http.StatusInternalServerError, codes.Internal)
	NotFound         = New(logrus.InfoLevel, "error.not_found", http.StatusNotFound, codes.NotFound)
	Invalid          = New(logrus.InfoLevel, "error.invalid", http.StatusBadRequest, codes.InvalidArgument)
	Unauthenticated  = New(logrus.InfoLevel, "error.auth.unauthenticated", http.StatusUnauthorized, codes.Unauthenticated)
	PermissionDenied = New(logrus.InfoLevel, "error.auth.permission_denied", http.StatusForbidden, codes.PermissionDenied)
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

func New(logLevel logrus.Level, msg string, httpStatus int, grpcCode codes.Code) Func {
	return func(ctx context.Context, reasons map[string]string, details ...string) error {
		errId := uuid.New().String()

		if log := middleware.GetLogger(ctx); log != nil {
			log.WithFields(logrus.Fields{
				"grpcCode":   grpcCode,
				"statusCode": httpStatus,
				"errorId":    errId,
				"details":    details,
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

func Error(ctx context.Context, logLevel logrus.Level, msg string, reasons map[string]string, httpStatus int, grpcCode codes.Code, details ...string) error {
	return New(logLevel, msg, httpStatus, grpcCode)(ctx, reasons, details...)
}

func (e exception) Error() string {
	return e.ErrorId
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
