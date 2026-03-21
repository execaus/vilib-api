package handler

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *Handler) GetPathStringValue(c *gin.Context, key int) (string, error) {
	value := c.Param(strconv.Itoa(key))
	if value == "" {
		zap.L().Error(fmt.Sprintf("parameter not found: %v", key))
		return "", errors.New(fmt.Sprintf("parameter not found: %v", key))
	}

	return value, nil
}
