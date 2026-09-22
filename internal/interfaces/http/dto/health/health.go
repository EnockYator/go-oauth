package health

type HealthDTO struct {
	Status      string `json:"status"`
	Application string `json:"application"`
}
