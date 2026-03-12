package dto

type RegisterRequest struct {
	Name    string `json:"name"    validate:"required,min=2,max=64"`
	Surname string `json:"surname" validate:"required,min=2,max=64"`
	Email   string `json:"email"   validate:"required,email,max=64"`
}

type RegisterResponse struct {
	Token string `json:"token"`
}
