package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/handler"
	"vilib-api/internal/saga"
	"vilib-api/internal/saga/saga_mocks"
	"vilib-api/internal/service"
	"vilib-api/testutil"

	"github.com/gojuno/minimock/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestHandler_ChangePassword(t *testing.T) {
	var (
		testUserID = uuid.New()
		testToken  = "valid-token"
	)

	setupCommitTx := func(mc *minimock.Controller) *saga_mocks.TransactableMock {
		tx := saga_mocks.NewBobTransactionMock(mc)
		tx.CommitMock.Expect(minimock.AnyContext).Return(nil)
		repo := saga_mocks.NewTransactableMock(mc)
		repo.WithTxMock.When(minimock.AnyContext).Then(tx, nil)
		return repo
	}

	setupRollbackTx := func(mc *minimock.Controller) *saga_mocks.TransactableMock {
		tx := saga_mocks.NewBobTransactionMock(mc)
		tx.RollbackMock.Expect(minimock.AnyContext).Return(nil)
		repo := saga_mocks.NewTransactableMock(mc)
		repo.WithTxMock.When(minimock.AnyContext).Then(tx, nil)
		return repo
	}

	url := "/api/v1/auth/password/change"

	newRequest := func(body dto.ChangePasswordRequest) *http.Request {
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+testToken)
		req.Header.Set("Content-Type", "application/json")
		return req
	}

	t.Run("success", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupCommitTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{UserID: testUserID}, nil)
		svcMock.Auth.ChangePasswordMock.
			When(minimock.AnyContext, testUserID, "old-password", "new-password").
			Then(nil)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		w := httptest.NewRecorder()
		router.ServeHTTP(
			w,
			newRequest(dto.ChangePasswordRequest{OldPassword: "old-password", NewPassword: "new-password"}),
		)

		require.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("old password invalid", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{UserID: testUserID}, nil)
		svcMock.Auth.ChangePasswordMock.
			When(minimock.AnyContext, testUserID, "wrong-old", "new-password").
			Then(service.ErrOldPasswordInvalid)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		w := httptest.NewRecorder()
		router.ServeHTTP(
			w,
			newRequest(dto.ChangePasswordRequest{OldPassword: "wrong-old", NewPassword: "new-password"}),
		)

		require.Equal(t, http.StatusBadRequest, w.Code)

		var resp dto.ErrorMessage
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, "validation.old_password", resp.Code)
	})

	t.Run("new password equals old", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{UserID: testUserID}, nil)
		svcMock.Auth.ChangePasswordMock.
			When(minimock.AnyContext, testUserID, "same-password", "same-password").
			Then(service.ErrPasswordInvalid)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		w := httptest.NewRecorder()
		router.ServeHTTP(
			w,
			newRequest(dto.ChangePasswordRequest{OldPassword: "same-password", NewPassword: "same-password"}),
		)

		require.Equal(t, http.StatusBadRequest, w.Code)

		var resp dto.ErrorMessage
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, "validation.password", resp.Code)
	})

	t.Run("unauthorized without token", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		payload, _ := json.Marshal(dto.ChangePasswordRequest{OldPassword: "old-password", NewPassword: "new-password"})
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid body", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Return(&domain.AuthClaims{UserID: uuid.New()}, nil)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(`{"old_password": ""}`)))
		req.Header.Set("Authorization", "Bearer "+testToken)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
