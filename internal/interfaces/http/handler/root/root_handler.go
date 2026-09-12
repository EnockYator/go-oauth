package root

import (
	"encoding/json"
	"net/http"
)

type RootResponse struct {
	Status string `json:"status"`
}

func RootHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_ = json.NewEncoder(w).Encode(RootResponse{
		Status: "ok",
	})
}
