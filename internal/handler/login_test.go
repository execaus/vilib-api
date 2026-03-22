package handler_test

import (
	"net/http"
	"testing"
	"vilib-api/internal/dto"
	"vilib-api/internal/handler"
	"vilib-api/testutil"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestLogin_Success(t *testing.T) {
	const (
		email    = "test@mail.ru"
		password = "password"
		token    = "token"
	)

	var response dto.LoginResponse

	code := testutil.RequestWithMocks(t, handler.APIVersion1).
		Method(http.MethodPost).
		Target(handler.LoginURL).
		Body(dto.LoginRequest{
			Password: password,
			Email:    email,
		}).
		PrepareService(func(t *testing.T, service *testutil.ServiceMock) {
			service.Auth.EXPECT().
				Login(gomock.Any(), email, password).
				Return(token, nil)
		}).
		Run(&response)

	require.Equal(t, http.StatusOK, code)
	require.Equal(t, token, response.Token)
}
