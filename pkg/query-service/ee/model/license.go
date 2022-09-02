package model

import (
	"go.signoz.io/query-service/model"
	"time"
)

type License struct {
	ActivationKey string     `json:"activationId" db:"activationKey"`
	Key           string     `json:"key" db:"key"`
	Scheme        string     `json:"scheme" db:"scheme"`
	PlanKey       string     `json:"planKey" db:"planKey"`
	Start         *time.Time `db:"start"`
	End           *time.Time `db:"end"`
	OrgId         string     `db:"org_id"`
	CreatedAt     time.Time  `db:"created_at"`
	UUID          string     `db:"uuid"`
}

func (l *License) ParseFeatures() (model.PlanFeatures, error) {
	// todo(amol): read license and extract billing plan
	// use billing plan to retrieve supported features from constants
	switch l.PlanKey {
	case "BasicPlan":
		return model.BasicPlan, nil
	case "ProPlan":
		return ProPlan, nil
	case "EnterprisePlan":
		return EnterprisePlan, nil
	}
	return ProPlan, nil
}
