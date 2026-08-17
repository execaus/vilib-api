package handler

import (
	"errors"

	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const headerAuthorization = "Authorization"

// RequireAuthMiddleware разбирает и проверяет JWT из заголовка Authorization вне транзакции
// саги (чистый CPU, без обращений к БД, §2.1 дизайна эпика). Принимает как значение вида
// "Bearer <jwt>", так и голый "<jwt>" — нормализацию делает Deps.Auth.GetClaimsFromToken.
// Валидные claims кладутся в контекст под claimsCtxKey; их читает h.claims().
func (h *Handler) RequireAuthMiddleware(c *gin.Context) {
	authHeader := c.GetHeader(headerAuthorization)
	if authHeader == "" {
		sendUnauthorized(c, codeUnauthorized, "authorization header is missing")
		return
	}

	claims, err := h.deps.Auth.GetClaimsFromToken(authHeader)
	if err != nil {
		zap.L().Warn(err.Error())

		if errors.Is(err, service.ErrTokenExpired) {
			sendUnauthorized(c, service.ErrTokenExpired.Code(), err.Error())
			return
		}

		sendUnauthorized(c, service.ErrTokenInvalid.Code(), service.ErrTokenInvalid.Error())
		return
	}

	c.Set(claimsCtxKey, claims)
}
