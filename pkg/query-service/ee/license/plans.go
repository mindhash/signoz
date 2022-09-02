package license

import (
	baseModel "go.signoz.io/query-service/model"
)

var ProPlan = baseModel.PlanFeatures{
	baseModel.SSO:  true,
	baseModel.SAML: true,
}

var EnterprisePlan = baseModel.PlanFeatures{
	baseModel.SSO:  true,
	baseModel.SAML: true,
}
