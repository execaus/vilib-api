package dto

type RegisterRequest struct {
	Name    string `json:"name"    binding:"required,min=2,max=64"`
	Surname string `json:"surname" binding:"required,min=2,max=64"`
	Email   string `json:"email"   binding:"required,email,max=64"`
}

type RegisterResponse struct {
	Token string `json:"token"`
}

type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email,max=64"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
}
