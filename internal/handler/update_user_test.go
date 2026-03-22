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

func TestUpdateUser_Success(t *testing.T) {
	var (
		name        = "name"
		surname     = "surname"
		email       = "test@mail.ru"
		password    = "password"
		initiatorID = "initiatorID"
		targetID    = "targetID"
		accountID   = "123"
		status      = domain.AccountAdminBitPosition
		claims      = domain.AuthClaims{
			UserID:           initiatorID,
			CurrentAccountID: accountID,
			Accounts:         []string{accountID},
		}
		token = "123"
	)

	var response dto.UpdateUserResponse

	code := testutil.RequestWithMocks(t, handler.APIVersion1).
		Method(http.MethodPut).
		Target(handler.UpdateUserURL.WithValues(targetID)).
		Body(dto.UpdateUserRequest{
			StatusPosition: &status,
		}).
		Authorization(token).
		PrepareService(func(t *testing.T, service *testutil.ServiceMock) {
			service.Auth.EXPECT().
				GetClaimsFromToken(token).
				Return(&claims, nil)
			service.User.EXPECT().
				Update(gomock.Any(), initiatorID, targetID, &status).
				Return(domain.User{
					ID:           targetID,
					Name:         name,
					Surname:      surname,
					PasswordHash: password,
					Email:        email,
					CreatedAt:    time.Now(),
				}, nil)
			service.AccountStatus.EXPECT().
				GetByUsersID(gomock.Any(), targetID).
				Return([]domain.AccountStatus{
					{
						AccountID: accountID,
						UserID:    targetID,
						Status:    1,
					},
				}, nil)
		}).
		Run(&response)

	require.Equal(t, http.StatusOK, code)
	require.Equal(t, targetID, response.User.ID)
	require.Equal(t, name, response.User.Name)
	require.Equal(t, surname, response.User.Surname)
	require.NotEqual(t, 0, response.User.Status)
}
