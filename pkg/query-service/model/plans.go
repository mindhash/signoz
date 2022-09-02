package model

type FeatureKey string

type PlanFeatures map[FeatureKey]bool

const SSO FeatureKey = "SSO"
const SAML FeatureKey = "SAML"

var BasicPlan = PlanFeatures{
	SSO:  false,
	SAML: false,
}
