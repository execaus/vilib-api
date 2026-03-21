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

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestRegister_Success(t *testing.T) {
	var response dto.RegisterResponse

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
			service.Auth.EXPECT().GeneratePassword().Return("generatedPassword", nil)
			service.Auth.EXPECT().HashPassword(gomock.Any(), "generatedPassword").Return("hashedPassword", nil)
			service.User.EXPECT().
				Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(domain.User{
				ID:        "userID",
				Name:      "userName",
				Surname:   "userSurname",
				Email:     "userEmail",
				CreatedAt: time.Now(),
			}, nil)
			service.Account.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(domain.Account{
				ID:        "accountID",
				Name:      "accountName",
				OwnerID:   "accountOwner",
				Email:     "accountEmail",
				CreatedAt: time.Now(),
			}, nil)
			service.User.EXPECT().IssueAdmin(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			service.Account.EXPECT().GetByUserID(gomock.Any(), gomock.Any()).Return([]domain.Account{}, nil)
			service.Auth.EXPECT().GenerateToken(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return("token", nil)
			service.Email.EXPECT().
				SendRegisteredMail(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, email string, pass string) error {
					password = pass
					localMailBox <- pass
					return nil
				})
		}).
		Run(&response)

	sentMail := <-localMailBox

	assert.Equal(t, password, sentMail)
	assert.Equal(t, http.StatusCreated, code)
	assert.NotEmpty(t, response.Token)
}
