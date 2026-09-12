package auth

type LoginRequest struct {
	Email    string `json:"email" example:"user@gmail.com"`
	Password string `json:"password" example:"secret123"`
}
