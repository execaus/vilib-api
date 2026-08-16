package handler

import (
	"vilib-api/internal/dto"

	"github.com/gin-gonic/gin"
)

// healthStatusOK — значение поля status в ответе успешной проверки работоспособности.
const healthStatusOK = "ok"

// Health godoc
// @Summary Проверка работоспособности сервиса
// @Description Возвращает 200, если процесс API запущен и обрабатывает запросы; используется healthcheck'ом контейнера
// @Tags health
// @Produce json
// @Success 200 {object} dto.HealthResponse
// @Router /api/health [get]
func (h *Handler) Health(c *gin.Context) {
	sendOK(c, dto.HealthResponse{Status: healthStatusOK})
}
