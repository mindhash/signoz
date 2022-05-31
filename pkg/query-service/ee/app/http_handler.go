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
		ch:         reader,
		qsRepo:     qsrepo,
		lm:         lm,
		APIHandler: baseHandler,
	}
	return ah, nil
}

// RegisterRoutes registers routes for this handler on the given router
func (aH *APIHandler) RegisterRoutes(router *mux.Router) {
	aH.APIHandler.RegisterRoutes(router)

	// ee specific routes
	router.HandleFunc("/api/v1/organization/{org_id}/complete/saml", baseApp.OpenAccess(aH.ReceiveSAML)).Methods(http.MethodPost)

	// base overrides
	router.HandleFunc("/api/v1/version", baseApp.OpenAccess(aH.getVersion)).Methods(http.MethodGet)

}

func (aH *APIHandler) getVersion(w http.ResponseWriter, r *http.Request) {
	version := version.GetVersion()
	aH.WriteJSON(w, r, map[string]string{"version": version, "eeAvailable": "Y"})
}
