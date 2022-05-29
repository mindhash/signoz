package license

import (
	"time"
	"go.signoz.io/query-service/model"
)

type License {
	Key string
	Start time.Datetime
	End time.Datetime
	// optional in single org?
	Org *model.Organization
}