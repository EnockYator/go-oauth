package health

import (
	"net/http"

	healthdto "github.com/EnockYator/go-oauth/internal/interfaces/http/dto/health"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/response"
	"github.com/EnockYator/go-oauth/internal/shared/apperror"
)

// HealthCheck godoc
//
// @Summary Health check
// @Description Returns service health status
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} healthdto.HealthResponse
// @Failure 405 {object} response.APIResponse
// @Router /health [get]
func Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.WriteError(
			w,
			r,
			apperror.New(
				r.Context(),
				apperror.CodeMethodNotAllowed,
				"method not allowed",
				nil,
			),
		)
		return
	}

	response.WriteResponse(
		w,
		http.StatusOK,
		healthdto.HealthResponse{
			Status:      "ok",
			Application: "go-oauth",
		},
	)
}
