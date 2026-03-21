package handler

import (
	"github.com/gin-gonic/gin"
)

const (
	headerAuthorization = "Authorization"
	authorizationCtxKey = "auth"
)

func (h *Handler) RequireAuthMiddleware(c *gin.Context) {
	authHeader := c.GetHeader(headerAuthorization)
	if authHeader == "" {
		sendUnauthorized(c)
		return
	}

	c.Set(authorizationCtxKey, authHeader)
}
