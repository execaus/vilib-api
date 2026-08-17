package handler

import (
	"errors"
	"net/http"
	"vilib-api/internal/dto"
	"vilib-api/internal/repository"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// sendServiceError конвертирует ошибку сервисного слоя в HTTP-ответ с телом
// dto.ErrorMessage{code, message} (§6.8 ТЗ). Код ошибки достаётся через [errors.As] у
// известных типов ошибок; HTTP 500 остаётся с пустым телом.
func sendServiceError(c *gin.Context, err error) {
	var conflictError service.ConflictError
	var forbiddenError *service.ForbiddenError
	var unauthorizedError *service.UnauthorizedError
	var validationError *service.ValidationError

	if errors.Is(err, repository.ErrNotFound) || errors.Is(err, service.ErrNotFound) {
		sendNotFound(c, codeNotFound, messageNotFound)
		return
	}

	if errors.As(err, &conflictError) {
		sendConflict(c, conflictError.Code(), conflictError.Error())
		return
	}

	if errors.As(err, &forbiddenError) {
		sendForbidden(c, forbiddenError.Code(), forbiddenError.Error())
		return
	}

	if errors.As(err, &unauthorizedError) {
		sendUnauthorized(c, unauthorizedError.Code(), unauthorizedError.Error())
		return
	}

	if errors.As(err, &validationError) {
		sendConflictOrValidation(c, validationError.Code(), validationError.Error())
		return
	}

	sendInternalError(c, err)
}

// sendConflictOrValidation отправляет HTTP 400 с телом dto.ErrorMessage — вынесено отдельной
// функцией, чтобы не путать с sendBadRequest (ошибки биндинга Gin, код всегда codeValidation).
func sendConflictOrValidation(c *gin.Context, code, message string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, &dto.ErrorMessage{Code: code, Message: message})
}

// sendServiceErrorWithDeactivated — вариант sendServiceError для Login: деактивированная
// строка пользователя (service.ErrUserDeactivated) на входе — это не конфликт состояния,
// а недействительная сессия (HTTP 401 user_deactivated, §2.2 дизайна эпика), поэтому
// обрабатывается отдельно от общего 409-маппинга ConflictError.
func sendServiceErrorWithDeactivated(c *gin.Context, err error) {
	if errors.Is(err, service.ErrUserDeactivated) {
		sendUnauthorized(c, codeUserDeactivated, messageUserDeactivated)
		return
	}
	sendServiceError(c, err)
}

// sendBadRequest отправляет HTTP 400 для ошибок биндинга Gin (path/query/JSON) — код всегда
// codeValidation, сообщение — текст ошибки биндинга.
func sendBadRequest(c *gin.Context, err error) {
	c.AbortWithStatusJSON(http.StatusBadRequest, &dto.ErrorMessage{Code: codeValidation, Message: err.Error()})
}

func sendInternalError(c *gin.Context, err error) {
	zap.L().Error(err.Error())
	c.AbortWithStatus(http.StatusInternalServerError)
}

func sendConflict(c *gin.Context, code, message string) {
	c.AbortWithStatusJSON(http.StatusConflict, &dto.ErrorMessage{Code: code, Message: message})
}

func sendCreated(c *gin.Context, body any) {
	c.AbortWithStatusJSON(http.StatusCreated, body)
}

func sendOK(c *gin.Context, body any) {
	c.AbortWithStatusJSON(http.StatusOK, body)
}

func sendUnauthorized(c *gin.Context, code, message string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, &dto.ErrorMessage{Code: code, Message: message})
}

func sendForbidden(c *gin.Context, code, message string) {
	c.AbortWithStatusJSON(http.StatusForbidden, &dto.ErrorMessage{Code: code, Message: message})
}

func sendNotFound(c *gin.Context, code, message string) {
	c.AbortWithStatusJSON(http.StatusNotFound, &dto.ErrorMessage{Code: code, Message: message})
}

func sendNoContent(c *gin.Context) {
	c.AbortWithStatus(http.StatusNoContent)
}
