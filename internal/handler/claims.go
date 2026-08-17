package handler

import (
	"vilib-api/internal/domain"

	"github.com/gin-gonic/gin"
)

// claimsCtxKey — ключ контекста запроса, под которым RequireAuthMiddleware кладёт
// разобранные claims JWT.
const claimsCtxKey = "claims"

// claims — единая точка чтения claims текущего запроса, разобранных RequireAuthMiddleware до
// открытия транзакции саги. Отсутствие claims в контексте — программная ошибка (handler вызван
// без RequireAuthMiddleware на маршруте) и превращается в HTTP 500.
func (h *Handler) claims(c *gin.Context) (*domain.AuthClaims, error) {
	value, ok := c.Get(claimsCtxKey)
	if !ok {
		return nil, ErrClaimsContextEmpty
	}

	claims, ok := value.(*domain.AuthClaims)
	if !ok {
		return nil, ErrClaimsContextEmpty
	}

	return claims, nil
}
