package middleware

import (
	"context"
	"github.com/stretchr/testify/suite"
	"net/http"
	"testing"
)

type DecoderTestSuite struct {
	suite.Suite
}

func (s *DecoderTestSuite) TestParseBool() {
	request, err := http.NewRequest("GET", "https://my.domain.com/api/call?param1&param2=val2&param3&param4=val4&param5", nil)
	s.NoError(err)
	ctx := context.Background()
	requestAfter, err := DecoderParseBool(func(_ context.Context, r *http.Request) (request interface{}, err error) {
		return r, nil
	})(ctx, request)
	s.NoError(err)
	request = requestAfter.(*http.Request)
	s.Equal("param1=true&param2=val2&param3=true&param4=val4&param5=true", request.URL.RawQuery)
	s.Equal("true", request.URL.Query().Get("param1"))
	s.Equal("val2", request.URL.Query().Get("param2"))
	s.Equal("true", request.URL.Query().Get("param3"))
	s.Equal("val4", request.URL.Query().Get("param4"))
	s.Equal("true", request.URL.Query().Get("param5"))
}

func TestDecoderTestSuite(t *testing.T) {
	suite.Run(t, &DecoderTestSuite{})
}
