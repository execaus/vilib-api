package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/handler"
	"vilib-api/internal/saga"
	"vilib-api/internal/saga/saga_mocks"
	"vilib-api/testutil"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/require"
)

func TestHandler_Register(t *testing.T) {
	var (
		testName    = "John"
		testSurname = "Doe"
		testEmail   = "john@example.com"
	)

	t.Run("success", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Account.CreateMock.Expect(minimock.AnyContext, testName, testSurname, testEmail).Return(domain.Account{}, nil)

		router := testutil.SetupTestRouterWithMocks(mc, svcMock, nil)

		body, _ := json.Marshal(dto.RegisterRequest{
			Name:    testName,
			Surname: testSurname,
			Email:   testEmail,
		})

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Account.CreateMock.Expect(minimock.AnyContext, testName, testSurname, testEmail).Return(domain.Account{}, errors.New("test error"))

		tx := saga_mocks.NewBobTransactionMock(mc)
		tx.RollbackMock.Expect(minimock.AnyContext).Return(nil)

		repo := saga_mocks.NewTransactableMock(mc)
		repo.WithTxMock.Expect(minimock.AnyContext).Return(tx, nil)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo))
		router := h.GetRouter()

		body, _ := json.Marshal(dto.RegisterRequest{
			Name:    testName,
			Surname: testSurname,
			Email:   testEmail,
		})

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
