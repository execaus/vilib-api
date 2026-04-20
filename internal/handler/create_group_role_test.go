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
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestHandler_CreateGroupRole(t *testing.T) {
	var (
		testAccountID   = uuid.New()
		testInitiatorID = uuid.New()
		testToken       = "valid-token"
		testName        = "Admin"
		testPermission  = domain.PermissionMask(1)
		testRole        = domain.GroupRole{
			ID:        uuid.New(),
			AccountID: testAccountID,
			Name:      testName,
		}
	)

	t.Run("success", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)

		tx := saga_mocks.NewBobTransactionMock(mc)
		tx.CommitMock.Expect(minimock.AnyContext).Return(nil)

		repo := saga_mocks.NewTransactableMock(mc)
		repo.WithTxMock.Expect(minimock.AnyContext).Return(tx, nil)

		svcMock.Auth.GetClaimsFromTokenMock.Expect("Bearer "+testToken).Return(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.GroupRole.CreateMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID, testName, testPermission, false).Return(testRole, nil)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo))
		router := h.GetRouter()

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/roles"
		body, _ := json.Marshal(dto.CreateGroupRoleRequest{
			Name:           testName,
			PermissionMask: testPermission,
			IsDefault:      false,
		})
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("invalid account id", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		url := "/api/v1/accounts/invalid-uuid/user-groups/roles"
		req := httptest.NewRequest(http.MethodPost, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/roles"
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)

		tx := saga_mocks.NewBobTransactionMock(mc)
		tx.RollbackMock.Expect(minimock.AnyContext).Return(nil)

		repo := saga_mocks.NewTransactableMock(mc)
		repo.WithTxMock.Expect(minimock.AnyContext).Return(tx, nil)

		svcMock.Auth.GetClaimsFromTokenMock.Expect("Bearer "+testToken).Return(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.GroupRole.CreateMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID, testName, testPermission, false).Return(domain.GroupRole{}, errors.New("test error"))

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo))
		router := h.GetRouter()

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/roles"
		body, _ := json.Marshal(dto.CreateGroupRoleRequest{
			Name:           testName,
			PermissionMask: testPermission,
			IsDefault:      false,
		})
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
