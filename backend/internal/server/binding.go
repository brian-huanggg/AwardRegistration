package server

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// Mount registers the generated routes under /api/v1 with the PRD's binding-error mapping.
func Mount(si api.ServerInterface) func(chi.Router) {
	return func(r chi.Router) {
		api.HandlerWithOptions(si, api.ChiServerOptions{
			BaseURL:          "/api/v1",
			BaseRouter:       r,
			ErrorHandlerFunc: BindingError,
		})
	}
}

// BindingError maps oapi-codegen parameter-binding failures to the PRD's codes
// (Backend.md §3): a malformed path ID is the uniform 404 (never 400); a malformed
// query value is a 422 with a field error; anything else is a client bug (400).
func BindingError(w http.ResponseWriter, r *http.Request, err error) {
	var format *api.InvalidParamFormatError
	var required *api.RequiredParamError
	var many *api.TooManyValuesForParamError

	switch {
	case errors.As(err, &format):
		if format.ParamName == "id" {
			web.WriteProblem(w, r, web.NewError(web.CodeNotFound, "resource not found"))
			return
		}
		writeField(w, r, format.ParamName, web.CodeInvalidFormat)
	case errors.As(err, &required):
		writeField(w, r, required.ParamName, web.CodeRequired)
	case errors.As(err, &many):
		writeField(w, r, many.ParamName, web.CodeInvalidFormat)
	default:
		web.WriteProblem(w, r, web.NewError(web.CodeMalformedRequest, "malformed request"))
	}
}

func writeField(w http.ResponseWriter, r *http.Request, field string, code web.ErrorCode) {
	e := web.NewError(web.CodeValidationFailed, "invalid parameter")
	e.Fields = []web.FieldError{{Field: field, Code: code}}
	web.WriteProblem(w, r, e)
}
