package license

import (
	"go.signoz.io/query-service/constants"
	"go.signoz.io/query-service/model"
	"time"
)

type License struct {
	Key   string        `db:"key"`
	Start time.Datetime `db:"start"`
	End   time.Datetime `db:"end"`
	Org   *model.Organization
	OrgID string `db:"uuid"`
	UUID  string `db:"uuid"`
}

func (l *License) LoadFeatures() (*SupportedFeatures, error) {
	// read license and extract billing plan
	// use billing plan to retrieve supported features from constants
	return &constants.EnterprisePlan
}
