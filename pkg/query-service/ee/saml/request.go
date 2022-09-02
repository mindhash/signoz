package saml

import (
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"go.signoz.io/query-service/constants"
	"go.signoz.io/query-service/model"

	saml2 "github.com/russellhaering/gosaml2"
	dsig "github.com/russellhaering/goxmldsig"
)

func BuildLoginURLWithOrg(org *model.Organization, pathToVisit string) (loginURL string, err error) {
	sp, err := PrepRequestWithOrg(org, pathToVisit)
	if err != nil {
		return "", err
	}
	return sp.BuildAuthURL(pathToVisit)
}

// PrepareAcsURL is the api endpoint that saml provider redirects response to
func PrepareAcsURL(orgID string) string {
	return fmt.Sprintf("%s/%s/organization/%s/%s", constants.GetSAMLHost(), "api/v1", orgID, "complete/saml")
}

// PrepRequestWithOrg prepares authorization URL (Idp Provider URL) using
// org setup
func PrepRequestWithOrg(org *model.Organization, pathToVisit string) (*saml2.SAMLServiceProvider, error) {

	certStore := dsig.MemoryX509CertificateStore{
		Roots: []*x509.Certificate{},
	}
	certData, err := base64.StdEncoding.DecodeString(org.GetSAMLCert())
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("failed to prepare saml login request: %v", err))
	}

	idpCert, err := x509.ParseCertificate(certData)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("failed to prepare saml login request, invalid cert: %s", err.Error()))
	}
	certStore.Roots = append(certStore.Roots, idpCert)
	acsURL := PrepareAcsURL(org.Id)

	// We sign the AuthnRequest with a random key because Okta doesn't seem
	// to verify these.
	randomKeyStore := dsig.RandomKeyStoreForTest()

	sp := &saml2.SAMLServiceProvider{
		IdentityProviderSSOURL:      org.GetSAMLIdpURL(),
		IdentityProviderIssuer:      org.GetSAMLEntityID(),
		ServiceProviderIssuer:       "urn:example:idp",
		AssertionConsumerServiceURL: acsURL,
		SignAuthnRequests:           true,
		AudienceURI:                 constants.GetSAMLHost(),
		IDPCertificateStore:         &certStore,
		SPKeyStore:                  randomKeyStore,
	}

	return sp, nil
}
