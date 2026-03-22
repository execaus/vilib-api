package dto

import (
	"vilib-api/internal/domain"
)

type CreateUserRequest struct {
	Name    string `json:"name"    binding:"required,min=2,max=64"`
	Surname string `json:"surname" binding:"required,min=2,max=64"`
	Email   string `json:"email"   binding:"required,email,max=64"`
}

type CreateUserResponse struct {
	User User `json:"user"`
}

type UpdateUserRequest struct {
	StatusPosition *domain.BitPosition `json:"status_position"`
}

type UpdateUserResponse struct {
	User User
}

type User struct {
	ID      string             `json:"id"`
	Name    string             `json:"name"`
	Surname string             `json:"surname"`
	Email   string             `json:"email"`
	Status  domain.BitmapValue `json:"status"`
}

func (u *User) FromDomain(user domain.User, status domain.BitmapValue) {
	u.ID = user.ID
	u.Name = user.Name
	u.Email = user.Email
	u.Surname = user.Surname
	u.Status = status
}
