package handler

import (
	"github.com/gin-gonic/gin"
)

// configCacheControl — значение заголовка Cache-Control ответа: конфиг статичен на время
// жизни процесса (собирается один раз при старте из config.Config), поэтому клиент и
// промежуточные кэши могут переиспользовать ответ короткое время (§5.2 контракта Э2, П-8).
const configCacheControl = "public, max-age=60"

// GetConfig godoc
// @Summary Публичный конфиг фронтенда
// @Description Возвращает лимиты и параметры загрузки видео, время жизни ссылок и токенов,
// @Description набор профилей качества HLS — без авторизации, чтобы форма загрузки могла
// @Description валидировать файл до входа в систему не дублируя константы на фронте.
// @Tags config
// @Produce json
// @Success 200 {object} dto.ConfigResponse
// @Router /api/v1/config [get]
func (h *Handler) GetConfig(c *gin.Context) {
	c.Header("Cache-Control", configCacheControl)
	sendOK(c, h.deps.PublicConfig)
}
