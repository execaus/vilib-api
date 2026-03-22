package handler_test

import (
	"context"
	"net/http"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/testutil"

	"testing"
	"vilib-api/internal/dto"
	"vilib-api/internal/handler"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRegister_Success(t *testing.T) {
	var (
		localMailBox = make(chan string, 1)
		password     string
	)

	code := testutil.RequestWithMocks(t, handler.APIVersion1).
		Method(http.MethodPost).
		Target(handler.RegisterURL).
		Body(dto.RegisterRequest{
			Name:    "Name",
			Surname: "Surname",
			Email:   "test@mail.ru",
		}).
		LocalMailBox(localMailBox).
		PrepareService(func(t *testing.T, service *testutil.ServiceMock) {
			service.
		}).

	require.Equal(t, http.StatusCreated, code)
	require.NotEmpty(t, response.Token)
}
