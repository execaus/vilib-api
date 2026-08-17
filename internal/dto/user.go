package dto

import (
	"time"
	"vilib-api/internal/domain"

	"github.com/google/uuid"
)

type CreateUserRequest struct {
	Name    string `json:"name"    binding:"required,min=2,max=64"`
	Surname string `json:"surname" binding:"required,min=2,max=64"`
	Email   string `json:"email"   binding:"required,email,max=64"`
}

type CreateUserResponse struct {
	User User `json:"user"`
}

// UpdateUserRequest — тело PUT accounts/{accountId}/users/{userId}: частичное обновление
// (§4 дизайна эпика Э2, «Блок C — редактирование»). Nil-поле оставляет значение без изменений;
// все поля nil — 200 без изменений. Смена роли (RoleID) и правка чужого профиля требуют права
// ManageUsers; инициатор, правящий собственное ФИО без смены роли, — исключение без прав.
type UpdateUserRequest struct {
	Name    *string    `json:"name"    binding:"omitempty,min=2,max=64"`
	Surname *string    `json:"surname" binding:"omitempty,min=2,max=64"`
	RoleID  *uuid.UUID `json:"role_id"`
}

type UpdateUserResponse struct {
	User User `json:"user"`
}

type GetAllUsersRequest struct {
	Status string `form:"status"`
}

type GetAllUsersResponse struct {
	Users []User `json:"users"`
}

type User struct {
	ID            uuid.UUID  `json:"id"`
	Name          string     `json:"name"`
	Surname       string     `json:"surname"`
	Email         string     `json:"email"`
	RoleID        uuid.UUID  `json:"role_id"`
	Status        string     `json:"status"`
	DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
}

func (u *User) FromDomain(user domain.User) {
	u.ID = user.ID
	u.Name = user.Name
	u.Email = user.Email
	u.Surname = user.Surname
	u.RoleID = user.RoleID
	if user.IsActive() {
		u.Status = "active"
	} else {
		u.Status = "deactivated"
		u.DeactivatedAt = user.DeactivatedAt
	}
}
