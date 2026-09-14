package health

type HealthResponse struct {
	Status      string `json:"status"`
	Application string `json:"application"`
}
