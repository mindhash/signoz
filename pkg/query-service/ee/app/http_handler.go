package app

import (
	"github.com/gorilla/mux"
	baseApp "go.signoz.io/query-service/app"
	eeDao "go.signoz.io/query-service/ee/dao"
	"net/http"
)

type APIHandler struct {
	ch     EventReader
	qsRepo eeDao.ModelDao
	*baseApp.APIHandler
}

// NewAPIHandler returns an APIHandler
func NewAPIHandler(reader EventReader, qsrepo eeDao.ModelDao) (*APIHandler, error) {
	baseHandler, err := baseApp.NewAPIHandler(reader, qsrepo, "../config/dashboards")
	if err != nil {
		return nil, err
	}
	ah := &APIHandler{
		ch:         reader,
		qsRepo:     qsrepo,
		APIHandler: baseHandler,
	}
	return ah, nil
}

// RegisterRoutes registers routes for this handler on the given router
func (aH *APIHandler) RegisterRoutes(router *mux.Router) {
	aH.APIHandler.RegisterRoutes(router)
	router.HandleFunc("/api/v1/organization/{org_id}/complete/saml", baseApp.OpenAccess(aH.ReceiveSAML)).Methods(http.MethodPost)
}
