package app

import (
	"github.com/gorilla/mux"
	baseApp "go.signoz.io/query-service/app"

	eeDao "go.signoz.io/query-service/ee/dao"
	license "go.signoz.io/query-service/ee/license"
	"go.signoz.io/query-service/version"
	"net/http"
)

type APIHandler struct {
	ch             EventReader
	qsRepo         eeDao.ModelDao
	licenseManager *license.LicenseManager
	*baseApp.APIHandler
}

// NewAPIHandler returns an APIHandler
func NewAPIHandler(reader EventReader, qsrepo eeDao.ModelDao, lm *license.LicenseManager) (*APIHandler, error) {
	baseHandler, err := baseApp.NewAPIHandler(reader, qsrepo, "../config/dashboards")
	if err != nil {
		return nil, err
	}
	ah := &APIHandler{
		ch:             reader,
		qsRepo:         qsrepo,
		licenseManager: lm,
		APIHandler:     baseHandler,
	}
	return ah, nil
}

// RegisterRoutes registers routes for this handler on the given router
func (ah *APIHandler) RegisterRoutes(router *mux.Router) {
	// note: add ee override methods first

	// routes available only in ee version
	router.HandleFunc("/api/v1/apply/license", baseApp.AdminAccess(ah.applyLicense)).Methods(http.MethodPost)
	router.HandleFunc("/api/v1/featureFlags", baseApp.OpenAccess(ah.getFeatureFlags)).Methods(http.MethodGet)

	// paid plans specific routes
	router.HandleFunc("/api/v1/organization/{org_id}/complete/saml", baseApp.OpenAccess(ah.ReceiveSAML)).Methods(http.MethodPost)

	// base overrides
	router.HandleFunc("/api/v1/version", baseApp.OpenAccess(ah.getVersion)).Methods(http.MethodGet)

	ah.APIHandler.RegisterRoutes(router)

}

func (ah *APIHandler) getVersion(w http.ResponseWriter, r *http.Request) {
	version := version.GetVersion()
	ah.WriteJSON(w, r, map[string]string{"version": version, "eeAvailable": "Y"})
}
