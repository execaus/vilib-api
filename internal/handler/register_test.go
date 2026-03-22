package handler_test

import (
	"net/http"
	"vilib-api/internal/domain"
	"vilib-api/testutil"

	"testing"
	"vilib-api/internal/dto"
	"vilib-api/internal/handler"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	testName    = "Name"
	testSurname = "Surname"
	testEmail   = "test@mail.ru"
)

func TestRegister_Success(t *testing.T) {
	code := testutil.RequestWithMocks(t, handler.APIVersion1).
		Method(http.MethodPost).
		Target(handler.RegisterURL).
		Body(dto.RegisterRequest{
			Name:    testName,
			Surname: testSurname,
			Email:   testEmail,
		}).
		PrepareService(func(t *testing.T, service *testutil.ServiceMock) {
			service.Account.EXPECT().
				Create(gomock.Any(), testName, testSurname, testEmail).
				Return(domain.Account{}, nil)
		}).
		Run(nil)

	require.Equal(t, http.StatusCreated, code)
}
