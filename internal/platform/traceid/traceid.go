package traceid

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey struct{}

func New(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, uuid.NewString())
}

func From(ctx context.Context) string {
	id, ok := ctx.Value(ctxKey{}).(string)
	if !ok || id == "" {
		return "no-trace-id"
	}

	return id
}

type userKey struct{}

func WithUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, userKey{}, username)
}

func UsernameFrom(ctx context.Context) string {
	u, ok := ctx.Value(userKey{}).(string)
	if !ok {
		return ""
	}

	return u
}
