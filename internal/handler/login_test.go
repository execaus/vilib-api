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
	"github.com/stretchr/testify/require"
)

func TestHandler_Login(t *testing.T) {
	var (
		testEmail    = "john@example.com"
		testPassword = "password123"
		testToken    = "test-token"
	)

	t.Run("success", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.LoginMock.Expect(minimock.AnyContext, testEmail, testPassword).Return(testToken, nil)

		router := testutil.SetupTestRouterWithMocks(mc, svcMock, nil)

		body, _ := json.Marshal(dto.LoginRequest{
			Email:    testEmail,
			Password: testPassword,
		})

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
