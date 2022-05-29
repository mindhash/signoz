package app

import (
	baseApp "go.signoz.io/query-service/app"
)

type EventReader interface {
	Start()
	baseApp.Reader
}
