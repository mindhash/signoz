package rules

import (
	"context"
	"time"
)

type QueryFunc func(ctx context.Context, q string, t time.Time) (Vector, error)

type QueryOrigin struct{}

func NewQueryOriginContext(ctx context.Context, data map[string]interface{}) context.Context {
	return context.WithValue(ctx, QueryOrigin{}, data)
}
