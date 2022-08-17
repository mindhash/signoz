package constants

import (
  "signo.io/query-service/model"
)

const SSO FeatureKey = "SSO"
const SAML FeatureKey = "SAML"

var BasicPlan = PlanFeatures{
  SSO:  false,
  SAML: false,
}

var ProPlan = PlanFeatures{
  SSO:  true,
  SAML: true,
}

var EnterprisePlan = PlanFeatures{
  SSO:  true,
  SAML: true,
}
