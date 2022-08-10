package model

type Organization struct {
	Id              string `json:"id" db:"id"`
	Name            string `json:"name" db:"name"`
	CreatedAt       int64  `json:"createdAt" db:"created_at"`
	IsAnonymous     bool   `json:"isAnonymous" db:"is_anonymous"`
	HasOptedUpdates bool   `json:"hasOptedUpdates" db:"has_opted_updates"`
}

func (o *Organization) IsLDAPAvailable() bool {
	if !o.IsSSOEnabled() {
		return false
	}

	return true
}

func (o *Organization) GetLDAPConfig() LDAPConfig {
	return LDAPConfig{}
}

func (o *Organization) GetSAMLEntityID() string {
	return "urn:example:idp"
}

func (o *Organization) GetSAMLIdpURL() string {
	return "http://idp.oktadev.com"
}

func (o *Organization) GetSAMLCert() string {
	return "MIIDPDCCAiQCCQDydJgOlszqbzANBgkqhkiG9w0BAQUFADBgMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNU2FuIEZyYW5jaXNjbzEQMA4GA1UEChMHSmFua3lDbzESMBAGA1UEAxMJbG9jYWxob3N0MB4XDTE0MDMxMjE5NDYzM1oXDTI3MTExOTE5NDYzM1owYDELMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAUBgNVBAcTDVNhbiBGcmFuY2lzY28xEDAOBgNVBAoTB0phbmt5Q28xEjAQBgNVBAMTCWxvY2FsaG9zdDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMGvJpRTTasRUSPqcbqCG+ZnTAurnu0vVpIG9lzExnh11o/BGmzu7lB+yLHcEdwrKBBmpepDBPCYxpVajvuEhZdKFx/Fdy6j5mH3rrW0Bh/zd36CoUNjbbhHyTjeM7FN2yF3u9lcyubuvOzr3B3gX66IwJlU46+wzcQVhSOlMk2tXR+fIKQExFrOuK9tbX3JIBUqItpI+HnAow509CnM134svw8PTFLkR6/CcMqnDfDK1m993PyoC1Y+N4X9XkhSmEQoAlAHPI5LHrvuujM13nvtoVYvKYoj7ScgumkpWNEvX652LfXOnKYlkB8ZybuxmFfIkzedQrbJsyOhfL03cMECAwEAATANBgkqhkiG9w0BAQUFAAOCAQEAeHwzqwnzGEkxjzSD47imXaTqtYyETZow7XwBc0ZaFS50qRFJUgKTAmKS1xQBP/qHpStsROT35DUxJAE6NY1Kbq3ZbCuhGoSlY0L7VzVT5tpu4EY8+Dq/u2EjRmmhoL7UkskvIZ2n1DdERtd+YUMTeqYl9co43csZwDno/IKomeN5qaPc39IZjikJ+nUC6kPFKeu/3j9rgHNlRtocI6S1FdtFz9OZMQlpr0JbUt2T3xS/YoQJn6coDmJL5GTiiKM6cOe+Ur1VwzS1JEDbSS2TWWhzq8ojLdrotYLGd9JOsoQhElmz+tMfCFQUFLExinPAyy7YHlSiVX13QH2XTu/iQQ=="
}

func (o *Organization) IsSSOAvailable() bool {
	// todo(amol): does licence support SSO
	return o.IsSSOEnabled()
}

func (o *Organization) IsSAMLAvailable() bool {
	// todo(amol): does license support SAML
	return o.IsSAMLEnabled()
}

func (o *Organization) IsSSOEnabled() bool {
	return false
}

func (o *Organization) IsSAMLEnabled() bool {
	return false
}

type InvitationObject struct {
	Id        string `json:"id" db:"id"`
	Email     string `json:"email" db:"email"`
	Name      string `json:"name" db:"name"`
	Token     string `json:"token" db:"token"`
	CreatedAt int64  `json:"createdAt" db:"created_at"`
	Role      string `json:"role" db:"role"`
	OrgId     string `json:"orgId" db:"org_id"`
}

type User struct {
	Id                 string `json:"id" db:"id"`
	Name               string `json:"name" db:"name"`
	Email              string `json:"email" db:"email"`
	Password           string `json:"password,omitempty" db:"password"`
	CreatedAt          int64  `json:"createdAt" db:"created_at"`
	ProfilePirctureURL string `json:"profilePictureURL" db:"profile_picture_url"`
	OrgId              string `json:"orgId,omitempty" db:"org_id"`
	GroupId            string `json:"groupId,omitempty" db:"group_id"`
}

type UserPayload struct {
	User
	Role         string `json:"role"`
	Organization string `json:"organization"`
}

type Group struct {
	Id   string `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

type ResetPasswordRequest struct {
	Password string `json:"password"`
	Token    string `json:"token"`
}
