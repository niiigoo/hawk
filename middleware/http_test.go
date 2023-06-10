package middleware

import (
	"context"
	"github.com/stretchr/testify/suite"
	"net/http"
	"testing"
	"time"
)

type ServerTestSuite struct {
	suite.Suite
}

func (s *ServerTestSuite) TestRequestBearerFromCookie() {
	request, err := http.NewRequest("GET", "https://my.domain.com/api/call?param1&param2=val2", nil)
	s.NoError(err)

	request.AddCookie(&http.Cookie{
		Name:    "token",
		Value:   "my.jwt.token",
		Path:    "/",
		Domain:  "my.domain.com",
		Expires: time.Now().Add(time.Minute),
	})

	ctx := context.Background()
	ctx = requestBearerFromCookie("token")(ctx, request)
	val1 := ctx.Value("Authorization")
	val2 := ctx.Value("authorization")
	s.NotNil(val1)
	s.Equal("bearer my.jwt.token", val1)
	s.Equal(val1, val2)
}

func TestServerTestSuite(t *testing.T) {
	suite.Run(t, &ServerTestSuite{})
}
