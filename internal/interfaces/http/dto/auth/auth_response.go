package auth

import (
	"time"

	user "github.com/EnockYator/go-oauth/internal/modules/user/domain"
)

// UserResponse is the public representation of an authenticated user.
type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Provider  string    `json:"provider"`
	CreatedAt time.Time `json:"created_at"`
}

// NewUserResponse maps a domain user to its public DTO.
func NewUserResponse(u *user.User) UserResponse {
	if u == nil {
		return UserResponse{}
	}
	var avatar string
	if u.AvatarURL != nil {
		avatar = *u.AvatarURL
	}
	return UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		AvatarURL: avatar,
		Provider:  u.Provider,
		CreatedAt: u.CreatedAt,
	}
}
