package handler

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (h *Handler) GetPathUUIDValue(c *gin.Context, key PathKey) (uuid.UUID, error) {
	value := c.Param(strconv.FormatUint(uint64(key), 10))
	if value == "" {
		zap.L().Error(fmt.Sprintf("parameter not found: %v", key))
		return uuid.New(), fmt.Errorf("parameter not found: %v", key)
	}

	parsedValue, err := uuid.Parse(value)
	if err != nil {
		zap.L().Error(fmt.Sprintf("invalid parameter uuid: %s", parsedValue))
		return uuid.New(), fmt.Errorf("invalid parameter uuid: %w", err)
	}

	return parsedValue, nil
}

// GetPathStringValue возвращает строковое значение path-параметра как есть, без парсинга UUID
// (например, имя профиля HLS-плейлиста).
func (h *Handler) GetPathStringValue(c *gin.Context, key PathKey) (string, error) {
	value := c.Param(strconv.FormatUint(uint64(key), 10))
	if value == "" {
		zap.L().Error(fmt.Sprintf("parameter not found: %v", key))
		return "", fmt.Errorf("parameter not found: %v", key)
	}

	return value, nil
}
