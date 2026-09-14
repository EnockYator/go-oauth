package health

import (
	"net/http"

	"github.com/EnockYator/go-oauth/internal/interfaces/http/dto/health"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/response"
)

// LiveCheck godoc
//
// @Summary Health Live check
// @Description Returns service health live status
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} health.HealthResponse
// @Failure 500 {object} map[string]any
// @Router /health/live [get]
func Live(w http.ResponseWriter, r *http.Request) {
	response.WriteResponse(
		w,
		http.StatusOK,
		health.HealthResponse{
			Status:      "alive",
			Application: "go-oauth",
		},
	)
}
