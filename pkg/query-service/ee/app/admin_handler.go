package app

import (
	"context"
	"encoding/json"
	"fmt"
	baseApp "go.signoz.io/query-service/app"
	authpkg "go.signoz.io/query-service/auth"
	license "go.signoz.io/query-service/ee/license"
	"go.signoz.io/query-service/model"
	"net/http"
)

// methods to support admin api support

func (ah *APIHandler) applyLicense(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	var l license.License

	if err := json.NewDecoder(r.Body).Decode(&l); err != nil {
		baseApp.RespondError(w, &model.ApiError{
			Typ: model.ErrorBadData,
			Err: err,
		}, nil)
		return
	}

	if l.Key == "" {
		baseApp.RespondError(w, &model.ApiError{
			Typ: model.ErrorBadData,
			Err: fmt.Errorf("license key is required"),
		}, nil)
		return
	}

	user, err := authpkg.GetUserFromRequest(r)
	if err != nil {
		baseApp.RespondError(w, &model.ApiError{
			Typ: model.ErrorUnauthorized,
			Err: err,
		}, nil)
		return
	}

	org, apiError := ah.qsRepo.GetOrg(ctx, user.User.OrgId)
	if apiError != nil {
		baseApp.RespondError(w, apiError, nil)
		return
	}

	err = ah.licenseManager.ApplyLicense(l.Key, org)
	if err != nil {
		baseApp.RespondError(w, &model.ApiError{
			Typ: model.ErrorInternal,
			Err: err,
		}, nil)
		return
	}

	baseApp.WriteHttpResponse(w, map[string]string{"status": "success", "data": "license applied successfully"})

}
