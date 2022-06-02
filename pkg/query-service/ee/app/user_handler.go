package app

import (
	"fmt"
	baseApp "go.signoz.io/query-service/app"
	authpkg "go.signoz.io/query-service/auth"
	"go.signoz.io/query-service/constants"
	"go.signoz.io/query-service/model"
	"net/http"
)

// methods that use user session or jwt to return
// user specific data

func (ah *APIHandler) getFeatureFlags(w http.ResponseWriter, r *http.Request) {
	userPayload, err := authpkg.GetUserFromRequest(r)
	if err != nil {
		baseApp.RespondError(w, &model.ApiError{
			Typ: model.ErrorUnauthorized,
			Err: fmt.Errorf("action not available to this user"),
		}, nil)
		return
	}
	fmt.Println("user payload: ", userPayload)
	if userPayload.User.OrgId == "" {
		baseApp.RespondError(w, &model.ApiError{
			Typ: model.ErrorBadData,
			Err: fmt.Errorf("failed to determine user org, please contact signoz support"),
		}, nil)
		return
	}

	features, err := ah.licenseManager.GetFeatureFlags(userPayload.User.OrgId)
	if err != nil {
		baseApp.RespondError(w, &model.ApiError{
			Typ: model.ErrorInternal,
			Err: err,
		}, nil)
	}
	ah.WriteJSON(w, r, map[string]constants.SupportedFeatures{"features": features})
}
