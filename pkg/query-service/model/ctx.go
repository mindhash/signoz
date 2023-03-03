package model

import (
	"context"
	"fmt"
)

type ApiContextKey string

const (
	ApiContextKeyName = "apiContext"
)
const ApiContextUserId = "userId"

type ApiContext struct {
	M map[string]string
}

func (a ApiContext) Get(key string) string {
	return a.M[key]
}

func GetApiContextUserId(ctx context.Context) string {
	fmt.Println("ctx:", ctx)
	apiContext := ctx.Value(ApiContextKey(ApiContextKeyName))
	fmt.Println("did not found contexT:", apiContext)
	if apiContext == nil {
		return ""
	}

	a := apiContext.(ApiContext)

	return a.Get(ApiContextUserId)
}
