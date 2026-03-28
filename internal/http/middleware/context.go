package middleware

import (
	"context"

	"proxy-panel/internal/repository"
)

type contextKey string

const (
	ctxAdminKey contextKey = "admin"
)

func WithAdmin(ctx context.Context, admin repository.Admin) context.Context {
	return context.WithValue(ctx, ctxAdminKey, admin)
}

func AdminFromContext(ctx context.Context) (repository.Admin, bool) {
	admin, ok := ctx.Value(ctxAdminKey).(repository.Admin)
	return admin, ok
}

