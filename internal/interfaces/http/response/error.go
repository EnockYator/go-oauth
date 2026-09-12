package response

import (
	"net/http"

	"github.com/EnockYator/go-oauth/internal/shared/apperror"
)

func statusFromCode(code apperror.ErrorCode) int {
	status, ok := statusByCode[code]
	if !ok {
		return http.StatusInternalServerError
	}

	return status
}
