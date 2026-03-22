package handler_test

import (
	"net/http"
	"testing"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/handler"
	"vilib-api/testutil"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreateUser_Success(t *testing.T) {
	const (
		name      = "name"
		surname   = "surname"
		email     = "test@mail.ru"
		password  = "password"
		userID    = "userID"
		accountID = "123"
		status    = domain.BitmapValue(1)
	)

	var response dto.CreateUserResponse

	code := testutil.RequestWithMocks(t, handler.APIVersion1).
		Method(http.MethodPost).
		Target(handler.CreateUserURL.WithValues(accountID)).
		Body(dto.CreateUserRequest{
			Name:    name,
			Surname: surname,
			Email:   email,
		}).
		PrepareService(func(t *testing.T, service *testutil.ServiceMock) {
			service.Account.EXPECT().
				CreateUser(gomock.Any(), accountID, name, surname, email).
				Return(domain.User{
					ID:           userID,
					Name:         name,
					Surname:      surname,
					PasswordHash: password,
					Email:        email,
					CreatedAt:    time.Now(),
				}, nil)
			service.AccountStatus.EXPECT().
				GetByUsersID(gomock.Any(), userID).
				Return([]domain.AccountStatus{
					{
						AccountID: accountID,
						UserID:    userID,
						Status:    status,
					},
				}, nil)
		}).
		Run(&response)

	require.Equal(t, http.StatusCreated, code)
	require.Equal(t, userID, response.User.ID)
	require.Equal(t, name, response.User.Name)
	require.Equal(t, surname, response.User.Surname)
	require.Equal(t, status, response.User.Status)
}
