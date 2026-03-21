package handler_test

import (
	"net/http"
	"testing"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/handler"
	"vilib-api/testutil"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreateUser_Success(t *testing.T) {
	const (
		name     = "name"
		surname  = "surname"
		email    = "test@mail.ru"
		password = "password"
		userID   = "userID"
	)

	var response dto.CreateUserResponse

	code := testutil.RequestWithMocks(t, handler.APIVersion1).
		Method(http.MethodPost).
		Target(handler.CreateUserURL.WithValues("123")).
		Body(dto.CreateUserRequest{
			Name:    name,
			Surname: surname,
			Email:   email,
		}).
		PrepareService(func(t *testing.T, service *testutil.ServiceMock) {
			service.Auth.EXPECT().
				GeneratePassword().
				Return(password, nil)

			service.Account.EXPECT().
				GetByUserEmail(gomock.Any(), email).
				Return([]domain.Account{}, nil)

			service.User.EXPECT().
				Create(gomock.Any(), name, surname, email, password).
				Return(domain.User{ID: userID}, nil)

			service.User.EXPECT().
				IssueUser(gomock.Any(), userID, gomock.Any()).
				Return(nil)

			service.Email.EXPECT().
				SendCreateUserEmail(gomock.Any(), email, password).
				Return(nil)
		}).
		Run(&response)

	require.Equal(t, http.StatusCreated, code)
	require.Equal(t, response.User.ID, userID)
}
