package handler

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (h *Handler) GetPathUUIDValue(c *gin.Context, key PathKey) (uuid.UUID, error) {
	value := c.Param(strconv.FormatUint(uint64(key), 10))
	if value == "" {
		zap.L().Error(fmt.Sprintf("parameter not found: %v", key))
		return uuid.New(), errors.New(fmt.Sprintf("parameter not found: %v", key))
	}

	parsedValue, err := uuid.Parse(value)
	if err != nil {
		zap.L().Error(fmt.Sprintf("invalid parameter uuid: %s", parsedValue))
		return uuid.New(), fmt.Errorf("invalid parameter uuid: %s", err)
	}

	return parsedValue, nil
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
