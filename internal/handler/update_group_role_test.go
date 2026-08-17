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

func TestHandler_UpdateGroupRole(t *testing.T) {
	var (
		testAccountID   = uuid.New()
		testRoleID      = uuid.New()
		testInitiatorID = uuid.New()
		testToken       = "valid-token"
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

	requestBody := dto.UpdateGroupRoleRequest{
		Name:           "Тренер",
		PermissionMask: 3,
		IsDefault:      true,
	}

	newRequest := func(body dto.UpdateGroupRoleRequest) *http.Request {
		payload, _ := json.Marshal(body)
		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/roles/" + testRoleID.String()
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+testToken)
		req.Header.Set("Content-Type", "application/json")
		return req
	}

	t.Run("success", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupCommitTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.GroupRole.UpdateMock.
			When(
				minimock.AnyContext, testInitiatorID, testAccountID, testRoleID,
				requestBody.Name, requestBody.PermissionMask, requestBody.IsDefault,
			).
			Then(domain.GroupRole{ID: testRoleID, Name: requestBody.Name, IsDefault: true}, nil)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		w := httptest.NewRecorder()
		router.ServeHTTP(w, newRequest(requestBody))

		require.Equal(t, http.StatusOK, w.Code)

		var resp dto.UpdateGroupRoleResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, testRoleID, resp.Role.ID)
		require.True(t, resp.Role.IsDefault)
	})

	t.Run("conflict - default role required", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.GroupRole.UpdateMock.
			When(
				minimock.AnyContext, testInitiatorID, testAccountID, testRoleID,
				requestBody.Name, requestBody.PermissionMask, requestBody.IsDefault,
			).
			Then(domain.GroupRole{}, service.ErrDefaultRoleRequired)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		w := httptest.NewRecorder()
		router.ServeHTTP(w, newRequest(requestBody))

		require.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("conflict - duplicate name", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.GroupRole.UpdateMock.
			When(
				minimock.AnyContext, testInitiatorID, testAccountID, testRoleID,
				requestBody.Name, requestBody.PermissionMask, requestBody.IsDefault,
			).
			Then(domain.GroupRole{}, service.ErrGroupRoleNameExists)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		w := httptest.NewRecorder()
		router.ServeHTTP(w, newRequest(requestBody))

		require.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("not found", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.GroupRole.UpdateMock.
			When(
				minimock.AnyContext, testInitiatorID, testAccountID, testRoleID,
				requestBody.Name, requestBody.PermissionMask, requestBody.IsDefault,
			).
			Then(domain.GroupRole{}, service.ErrNotFound)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		w := httptest.NewRecorder()
		router.ServeHTTP(w, newRequest(requestBody))

		require.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("forbidden", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.GroupRole.UpdateMock.
			When(
				minimock.AnyContext, testInitiatorID, testAccountID, testRoleID,
				requestBody.Name, requestBody.PermissionMask, requestBody.IsDefault,
			).
			Then(domain.GroupRole{}, service.ErrForbidden)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		w := httptest.NewRecorder()
		router.ServeHTTP(w, newRequest(requestBody))

		require.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("invalid account id", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Return(
			&domain.AuthClaims{UserID: uuid.New(), CurrentAccountID: uuid.New()},
			nil,
		)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		payload, _ := json.Marshal(requestBody)
		url := "/api/v1/accounts/invalid-uuid/user-groups/roles/" + testRoleID.String()
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid body", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Return(
			&domain.AuthClaims{UserID: uuid.New(), CurrentAccountID: uuid.New()},
			nil,
		)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/roles/" + testRoleID.String()
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader([]byte(`{"name": ""}`)))
		req.Header.Set("Authorization", "Bearer "+testToken)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
