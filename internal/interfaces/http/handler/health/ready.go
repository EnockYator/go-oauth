package health

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/EnockYator/go-oauth/internal/interfaces/http/dto/health"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/response"
	"github.com/EnockYator/go-oauth/internal/shared/apperror"
)

// ReadinessCheck godoc
//
// @Summary Database readiness check
// @Description Returns database readiness status
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} health.HealthResponse
// @Failure 500 {object} map[string]any
// @Router /health/ready [get]
func Ready(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			response.WriteError(
				w,
				r,
				apperror.New(
					r.Context(),
					apperror.CodeServiceUnavailable,
					"method not allowed",
					nil,
				),
			)
			return
		}

		response.WriteResponse(
			w,
			http.StatusOK,
			health.HealthResponse{
				Status:      "ready",
				Application: "go-oauth-database",
			},
		)
	}
}
