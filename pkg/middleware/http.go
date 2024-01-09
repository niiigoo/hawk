package middleware

import (
	"context"
	kitHttp "github.com/go-kit/kit/transport/http"
	"net/http"
)

// RequestBearerFromCookie adds the value of a cookie (if present) as bearer token to the context using the key
// `Authorization`
func RequestBearerFromCookie(cookie string) kitHttp.ServerOption {
	return kitHttp.ServerBefore(requestBearerFromCookie(cookie))
}

func requestBearerFromCookie(cookie string) kitHttp.RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		c, err := r.Cookie(cookie)
		if err == nil {
			ctx = context.WithValue(ctx, "authorization", "bearer "+c.Value)
			ctx = context.WithValue(ctx, "Authorization", "bearer "+c.Value)
		}
		return ctx
	}
}

// CookieToContext adds the value of a cookie (if present) to the context using the provided name as key
func CookieToContext(cookie, name string) kitHttp.ServerOption {
	return kitHttp.ServerBefore(func(ctx context.Context, r *http.Request) context.Context {
		c, err := r.Cookie(cookie)
		if err == nil {
			ctx = context.WithValue(ctx, name, c.Value)
		}
		return ctx
	})
}
