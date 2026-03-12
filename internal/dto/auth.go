package dto

type RegisterRequest struct {
	Email string `json:"email" validate:"required,email,max=64"`
}

type RegisterResponse struct {
	Token string `json:"token"`
}
