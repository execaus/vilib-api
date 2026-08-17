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

func TestHandler_UpdateAccountRole(t *testing.T) {
	var (
		testAccountID   = uuid.New()
		testRoleID      = uuid.New()
		testInitiatorID = uuid.New()
		testToken       = "valid-token"
		testParentID    = uuid.New()
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

	requestBody := dto.UpdateAccountRoleRequest{
		Name:       "Менеджер",
		Permission: 2,
		ParentID:   &testParentID,
		IsDefault:  true,
	}

	newRequest := func(body dto.UpdateAccountRoleRequest) *http.Request {
		payload, _ := json.Marshal(body)
		url := "/api/v1/accounts/" + testAccountID.String() + "/roles/" + testRoleID.String()
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
		svcMock.AccountRole.UpdateMock.
			When(
				minimock.AnyContext, testInitiatorID, testAccountID, testRoleID,
				requestBody.Name, requestBody.ParentID, requestBody.Permission, requestBody.IsDefault,
			).
			Then(domain.AccountRole{ID: testRoleID, Name: requestBody.Name, IsDefault: true}, nil)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		w := httptest.NewRecorder()
		router.ServeHTTP(w, newRequest(requestBody))

		require.Equal(t, http.StatusOK, w.Code)

		var resp dto.UpdateAccountRoleResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, testRoleID, resp.AccountRole.ID)
		require.True(t, resp.AccountRole.IsDefault)
	})

	t.Run("conflict - system role", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.AccountRole.UpdateMock.
			When(
				minimock.AnyContext, testInitiatorID, testAccountID, testRoleID,
				requestBody.Name, requestBody.ParentID, requestBody.Permission, requestBody.IsDefault,
			).
			Then(domain.AccountRole{}, service.ErrIsSystemRole)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		w := httptest.NewRecorder()
		router.ServeHTTP(w, newRequest(requestBody))

		require.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("conflict - owner permission bit", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.AccountRole.UpdateMock.
			When(
				minimock.AnyContext, testInitiatorID, testAccountID, testRoleID,
				requestBody.Name, requestBody.ParentID, requestBody.Permission, requestBody.IsDefault,
			).
			Then(domain.AccountRole{}, service.ErrPermissionOwnerForbidden)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		w := httptest.NewRecorder()
		router.ServeHTTP(w, newRequest(requestBody))

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("conflict - default role required", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.AccountRole.UpdateMock.
			When(
				minimock.AnyContext, testInitiatorID, testAccountID, testRoleID,
				requestBody.Name, requestBody.ParentID, requestBody.Permission, requestBody.IsDefault,
			).
			Then(domain.AccountRole{}, service.ErrDefaultRoleRequired)

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
		svcMock.AccountRole.UpdateMock.
			When(
				minimock.AnyContext, testInitiatorID, testAccountID, testRoleID,
				requestBody.Name, requestBody.ParentID, requestBody.Permission, requestBody.IsDefault,
			).
			Then(domain.AccountRole{}, service.ErrAccountRoleNameExists)

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
		svcMock.AccountRole.UpdateMock.
			When(
				minimock.AnyContext, testInitiatorID, testAccountID, testRoleID,
				requestBody.Name, requestBody.ParentID, requestBody.Permission, requestBody.IsDefault,
			).
			Then(domain.AccountRole{}, service.ErrNotFound)

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
		svcMock.AccountRole.UpdateMock.
			When(
				minimock.AnyContext, testInitiatorID, testAccountID, testRoleID,
				requestBody.Name, requestBody.ParentID, requestBody.Permission, requestBody.IsDefault,
			).
			Then(domain.AccountRole{}, service.ErrForbidden)

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
		url := "/api/v1/accounts/invalid-uuid/roles/" + testRoleID.String()
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

		url := "/api/v1/accounts/" + testAccountID.String() + "/roles/" + testRoleID.String()
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader([]byte(`{"name": ""}`)))
		req.Header.Set("Authorization", "Bearer "+testToken)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
