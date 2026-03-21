package handler

import (
	"vilib-api/internal/domain"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func getClaims(c *gin.Context, services *service.Service) (*domain.AuthClaims, error) {
	token, ok := c.Get(authorizationCtxKey)
	if !ok {
		zap.L().Error(ErrAuthorizationContextEmpty.Error())
		return nil, ErrAuthorizationContextEmpty
	}

	strToken, ok := token.(string)
	if !ok {
		zap.L().Error(ErrInvalidCredentials.Error())
		return nil, ErrInvalidCredentials
	}

	claims, err := services.Auth.GetClaimsFromToken(c, strToken)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return claims, nil
}
