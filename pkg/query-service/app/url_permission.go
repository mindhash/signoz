package app

import (
	"errors"
	"github.com/gorilla/mux"
	"go.signoz.io/query-service/auth"
	"go.signoz.io/query-service/model"
	"net/http"
)

func OpenAccess(f func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f(w, r)
	}
}

func ViewAccess(f func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := auth.GetUserFromRequest(r)
		if err != nil {
			RespondError(w, &model.ApiError{
				Typ: model.ErrorUnauthorized,
				Err: err,
			}, nil)
			return
		}

		if !(auth.IsViewer(user) || auth.IsEditor(user) || auth.IsAdmin(user)) {
			RespondError(w, &model.ApiError{
				Typ: model.ErrorForbidden,
				Err: errors.New("API is accessible to viewers/editors/admins."),
			}, nil)
			return
		}
		f(w, r)
	}
}

func EditAccess(f func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := auth.GetUserFromRequest(r)
		if err != nil {
			RespondError(w, &model.ApiError{
				Typ: model.ErrorUnauthorized,
				Err: err,
			}, nil)
			return
		}
		if !(auth.IsEditor(user) || auth.IsAdmin(user)) {
			RespondError(w, &model.ApiError{
				Typ: model.ErrorForbidden,
				Err: errors.New("API is accessible to editors/admins."),
			}, nil)
			return
		}
		f(w, r)
	}
}

func SelfAccess(f func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := auth.GetUserFromRequest(r)
		if err != nil {
			RespondError(w, &model.ApiError{
				Typ: model.ErrorUnauthorized,
				Err: err,
			}, nil)
			return
		}
		id := mux.Vars(r)["id"]
		if !(auth.IsSelfAccessRequest(user, id) || auth.IsAdmin(user)) {
			RespondError(w, &model.ApiError{
				Typ: model.ErrorForbidden,
				Err: errors.New("API is accessible for self access or to the admins."),
			}, nil)
			return
		}
		f(w, r)
	}
}

func AdminAccess(f func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := auth.GetUserFromRequest(r)
		if err != nil {
			RespondError(w, &model.ApiError{
				Typ: model.ErrorUnauthorized,
				Err: err,
			}, nil)
			return
		}
		if !auth.IsAdmin(user) {
			RespondError(w, &model.ApiError{
				Typ: model.ErrorForbidden,
				Err: errors.New("API is accessible to admins only"),
			}, nil)
			return
		}
		f(w, r)
	}
}
