package pr

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Smana/app-wizard/internal/api"
	"github.com/Smana/app-wizard/internal/gitprovider"
	"github.com/Smana/app-wizard/internal/httputil"
)

// ProviderForRequest yields the gitprovider for the authenticated user of a
// request, or an error when unauthenticated.
type ProviderForRequest func(r *http.Request) (gitprovider.Provider, error)

// Handler serves POST /api/pr. Unauthenticated requests get 401; gate failures
// get 422 with the error; success returns api.PRResponse.
func (s *Service) Handler(providerFor ProviderForRequest, logger *slog.Logger) http.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(w http.ResponseWriter, r *http.Request) {
		provider, err := providerFor(r)
		if err != nil {
			httputil.WriteError(w, http.StatusUnauthorized, "not authenticated")
			return
		}

		var req api.PRRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		resp, err := s.Create(r.Context(), provider, req)
		if err != nil {
			if ge, ok := err.(*GateError); ok {
				logger.Info("pr gate blocked", "stack", req.Stack, "app", req.AppName, "reason", ge.Message)
				httputil.WriteJSON(w, http.StatusUnprocessableEntity, gateErrorResponse(ge))
				return
			}
			logger.Error("pr creation failed", "stack", req.Stack, "app", req.AppName, "err", err.Error())
			httputil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}

		logger.Info("pr created", "stack", req.Stack, "app", req.AppName, "number", resp.Number)
		httputil.WriteJSON(w, http.StatusCreated, resp)
	}
}

// gateErrorResponse builds the 422 body. When validation details exist it
// returns the ValidateResponse; otherwise a plain error envelope.
func gateErrorResponse(ge *GateError) any {
	if ge.Validate != nil {
		return ge.Validate
	}
	return api.ErrorResponse{Error: ge.Message}
}
