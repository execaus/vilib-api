package handler_test

import (
	"net/http"

	"testing"
	"vilib-api/internal/dto"
	"vilib-api/internal/handler"
	"vilib-api/internal/service"
	"vilib-api/test"

	"github.com/stretchr/testify/assert"
)

func TestRegisterHandler_Success_Registered(t *testing.T) {
	var response dto.RegisterResponse

	code := test.Request(
		t,
		http.MethodPost,
		handler.RegisterURL,
		&response,
		dto.RegisterRequest{Email: "test@mail.ru"},
		func(t *testing.T, service *service.Service) {
			// TODO
		},
	)

	// Проверяем статус
	assert.Equal(t, http.StatusCreated, code)
}

func TestRegisterHandler_UserExists_Conflict(t *testing.T) {
	// TODO
}
