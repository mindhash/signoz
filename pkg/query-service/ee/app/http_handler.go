package app

import (
	"github.com/gorilla/mux"
	baseApp "go.signoz.io/query-service/app"
	baseInt "go.signoz.io/query-service/interfaces"

	eeDao "go.signoz.io/query-service/ee/dao"
	license "go.signoz.io/query-service/ee/license"
	"go.signoz.io/query-service/version"
	"net/http"
)

type APIHandlerOptions struct {
	DataReader     EventReader
	ModelDao       eeDao.ModelDao
	FeatureFlags   baseInt.FeatureStore
	LicenseManager license.Manager
}

type APIHandler struct {
	opts Options
	*baseApp.APIHandler
}

// NewAPIHandler returns an APIHandler
func NewAPIHandler(opts Options) (*APIHandler, error) {
	baseHandler, err := baseApp.NewAPIHandler(opts.DataReader, opts.ModalDao, "../config/dashboards")
	if err != nil {
		return nil, err
	}
	ah := &APIHandler{
		opts:       opts,
		APIHandler: baseHandler,
	}
	return ah, nil
}

// RegisterRoutes registers routes for this handler on the given router
func (ah *APIHandler) RegisterRoutes(router *mux.Router) {
	// note: add ee override methods first

	// routes available only in ee version
	router.HandleFunc("/api/v1/apply/license", baseApp.AdminAccess(ah.applyLicense)).Methods(http.MethodPost)
	router.HandleFunc("/api/v1/featureFlags", baseApp.OpenAccess(ah.getFeatureFlags)).Methods(http.MethodGet)
	router.HandleFunc("/api/v1/loginPrecheck", OpenAccess(ah.precheckLogin)).Methods(http.MethodGet)

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

func (ah *APIHandler) CheckFeature(orgID string, f model.FeatureKey) error {
	if orgID == "" {
		userPayload, err := auth.GetUserFromRequest(r)
		if err != nil {
			return err
		}
		if userPayload.Organization != nil {
			orgID = userPayload.Organization.Id
		}
	}

	if orgID == "" {
		return fmt.Errorf("invalid org ID")
	}

	return ah.opts.FeatureFlags.CheckFeature(orgID, f)
}
