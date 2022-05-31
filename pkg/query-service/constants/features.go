package constants

type SupportedFeatures map[string]bool

var BasicPlan = SupportedFeatures{}

var ProPlan = SupportedFeatures{
	"SAML": false,
}

var EnterprisePlan = SupportedFeatures{
	"SAML": true,
}

const FEATURES_SAML = "SAML"
