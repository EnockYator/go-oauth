package root

import (
	"net/http"
	"os"

	"github.com/EnockYator/go-oauth/internal/interfaces/http/dto/root"
	"github.com/EnockYator/go-oauth/internal/interfaces/http/response"
	"github.com/EnockYator/go-oauth/internal/shared/apperror"
)

// Root godoc
//
// @Summary Root endpoint
// @Description Returns root endpoint
// @Tags Root
// @Accept json
// @Produce json
// @Success 200 {object} root.RootResponse
// @Failure 405 {object} response.APIErrorResponse
// @Router / [get]
func Root(w http.ResponseWriter, r *http.Request) {
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
		root.RootResponse{
			AppName: os.Getenv("APP_NAME"),
			AppVersion: os.Getenv("APP_VERSION"),
			AppEnv: os.Getenv("APP_ENV"),
			Status:      "ok",
		},
	)
}
