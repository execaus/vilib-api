package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vilib-api/internal/dto"
	"vilib-api/testutil"

	"github.com/gojuno/minimock/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestHandler_ForgotPassword(t *testing.T) {
	var testEmail = "john@example.com"

	t.Run("success without account id", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.RequestPasswordResetMock.
			Expect(minimock.AnyContext, testEmail, (*uuid.UUID)(nil)).
			Return(nil)

		router := testutil.SetupTestRouterWithMocks(mc, svcMock, nil)

		body, _ := json.Marshal(dto.ForgotPasswordRequest{Email: testEmail})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/forgot", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("success with account id", func(t *testing.T) {
		mc := minimock.NewController(t)

		testAccountID := uuid.New()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.RequestPasswordResetMock.
			Expect(minimock.AnyContext, testEmail, &testAccountID).
			Return(nil)

		router := testutil.SetupTestRouterWithMocks(mc, svcMock, nil)

		body, _ := json.Marshal(dto.ForgotPasswordRequest{Email: testEmail, AccountID: &testAccountID})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/forgot", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("unknown email is still 200", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.RequestPasswordResetMock.
			Expect(minimock.AnyContext, testEmail, (*uuid.UUID)(nil)).
			Return(nil)

		router := testutil.SetupTestRouterWithMocks(mc, svcMock, nil)

		body, _ := json.Marshal(dto.ForgotPasswordRequest{Email: testEmail})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/forgot", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var resp dto.ForgotPasswordResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	})

	t.Run("invalid json", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/forgot", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid email", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		body, _ := json.Marshal(dto.ForgotPasswordRequest{Email: "not-an-email"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/forgot", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
