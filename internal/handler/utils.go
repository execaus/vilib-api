package handler

import (
	"errors"
	"fmt"
	"reflect"
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

func sliceItemsToSingle[T1 any](fn func() ([]T1, error)) (T1, error) {
	t1, t2 := fn()
	val := reflect.ValueOf(t1)
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return *new(T1), errors.New("returned value is not a slice or array")
	}
	if len(t1) == 0 {
		return *new(T1), errors.New("slice is empty")
	}
	return t1[0], t2
}
