package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vilib-api/internal/dto"
	"vilib-api/internal/handler"
	"vilib-api/internal/saga"
	"vilib-api/internal/saga/saga_mocks"
	"vilib-api/internal/service"
	"vilib-api/testutil"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/require"
)

func TestHandler_ResetPassword(t *testing.T) {
	var (
		testToken       = "reset-token"
		testNewPassword = "new-password"
	)

	t.Run("success", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.ResetPasswordMock.
			Expect(minimock.AnyContext, testToken, testNewPassword).
			Return(nil)

		router := testutil.SetupTestRouterWithMocks(mc, svcMock, nil)

		body, _ := json.Marshal(dto.ResetPasswordRequest{Token: testToken, NewPassword: testNewPassword})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/reset", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("invalid token", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.ResetPasswordMock.
			Expect(minimock.AnyContext, testToken, testNewPassword).
			Return(service.ErrResetTokenInvalid)

		tx := saga_mocks.NewBobTransactionMock(mc)
		tx.RollbackMock.Expect(minimock.AnyContext).Return(nil)
		repo := saga_mocks.NewTransactableMock(mc)
		repo.WithTxMock.When(minimock.AnyContext).Then(tx, nil)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		body, _ := json.Marshal(dto.ResetPasswordRequest{Token: testToken, NewPassword: testNewPassword})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/reset", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)

		var resp dto.ErrorMessage
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, "validation.reset_token", resp.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/reset", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("password too short fails binding", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		body, _ := json.Marshal(dto.ResetPasswordRequest{Token: testToken, NewPassword: "short"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/reset", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
