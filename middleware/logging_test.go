package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/suite"
	"net/http"
	"testing"
)

type EndpointTestSuite struct {
	suite.Suite
}

func (s *EndpointTestSuite) TestEndpointLogging() {
	var output bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&output)
	log := logger.WithField("name", "test")

	request, err := http.NewRequest("GET", "https://my.domain.com/api/call?param1&param2=val2", nil)
	s.NoError(err)
	ctx := context.Background()
	_, err = EndpointLogging(log, nil)(
		"test",
		func(ctx context.Context, request interface{}) (response interface{}, err error) {
			return nil, nil
		},
	)(ctx, request)
	s.NoError(err)

	var out map[string]interface{}
	err = json.Unmarshal(output.Bytes(), &out)
	s.NoError(err)
	val, ok := out["transport_error"]
	s.True(ok)
	s.Nil(val)
	_, ok = out["took"]
	s.True(ok)
}

func (s *EndpointTestSuite) TestEndpointLoggingCtx() {
	var output bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&output)
	log := logger.WithField("name", "test")

	request, err := http.NewRequest("GET", "https://my.domain.com/api/call?param1&param2=val2", nil)
	s.NoError(err)
	ctx := context.Background()
	ctx = context.WithValue(ctx, "trace", "asdf-kjhg")
	ctx = context.WithValue(ctx, "keyNum", 123)
	ctx = context.WithValue(ctx, "key", "qwer")
	_, err = EndpointLogging(log, logrus.Fields{"trace-id": "trace", "num": "keyNum"})(
		"test",
		func(ctx context.Context, request interface{}) (response interface{}, err error) {
			return nil, nil
		},
	)(ctx, request)
	s.NoError(err)

	var out map[string]interface{}
	err = json.Unmarshal(output.Bytes(), &out)
	s.NoError(err)
	val, ok := out["trace-id"]
	s.True(ok)
	s.Equal("asdf-kjhg", val)
	val, ok = out["num"]
	s.True(ok)
	s.Equal(float64(123), val)
	val, ok = out["transport_error"]
	s.True(ok)
	s.Nil(val)
	_, ok = out["took"]
	s.True(ok)
}

func TestEndpointTestSuite(t *testing.T) {
	suite.Run(t, &EndpointTestSuite{})
}
