package middleware

import (
	"context"
	kitHttp "github.com/go-kit/kit/transport/http"

	"net/http"
	"regexp"
)

var (
	patternUrlQuery = regexp.MustCompile("(\\A|&)([^=&?]+)(&|\\z)")
)

// DecoderParseBool adds the value `true` to query parameters without a value.
func DecoderParseBool(next kitHttp.DecodeRequestFunc) kitHttp.DecodeRequestFunc {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		r.URL.RawQuery = patternUrlQuery.ReplaceAllString(r.URL.RawQuery, "$1$2=true$3")
		return next(ctx, r)
	}
}
