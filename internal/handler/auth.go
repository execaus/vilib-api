package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
)

func (h *Handler) Register(c *gin.Context) {
	var req dto.RegisterRequest

	if err := c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	var (
		token string
	)
	if err := h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		//services.Auth.GeneratePassword()
		//services.Account.Create()
		//services.User.Create()
		//services.Permission.IssueAdmin()
		//services.Email.SendPassword()

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendCreated(c, dto.RegisterResponse{Token: token})
}
