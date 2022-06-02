package license

import (
	"go.signoz.io/query-service/constants"
	"time"
)

type License struct {
	Key string `json:"key" db:"key"`
	// below attributes must be derived from parsing key
	Start     *time.Time `db:"start"`
	End       *time.Time `db:"end"`
	OrgId     string     `db:"org_id"`
	CreatedAt time.Time  `db:"created_at"`
	UUID      string     `db:"uuid"`
}

func (l *License) LoadFeatures() (constants.SupportedFeatures, error) {
	// read license and extract billing plan
	// use billing plan to retrieve supported features from constants
	return constants.EnterprisePlan, nil
}
