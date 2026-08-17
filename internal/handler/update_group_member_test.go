package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestHandler_UpdateGroupMember(t *testing.T) {
	var (
		testAccountID   = uuid.New()
		testGroupID     = uuid.New()
		testInitiatorID = uuid.New()
		testToken       = "valid-token"
		testUserID      = uuid.New()
		testRoleID      = uuid.New()
		testJoinedAt    = time.Now().UTC().Truncate(time.Second)
		testMember      = domain.GroupMemberDetails{
			UserID:         testUserID,
			Name:           "Иван",
			Surname:        "Иванов",
			Email:          "ivan@example.com",
			RoleID:         testRoleID,
			RoleName:       "Менеджер",
			PermissionMask: domain.SetBits(0, domain.GroupPermissionManageMembers),
			JoinedAt:       testJoinedAt,
		}
	)

	t.Run("success", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)

		tx := saga_mocks.NewBobTransactionMock(mc)
		tx.CommitMock.Expect(minimock.AnyContext).Return(nil)
		repo := saga_mocks.NewTransactableMock(mc)
		repo.WithTxMock.When(minimock.AnyContext).Then(tx, nil)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.GroupMember.UpdateRoleMock.
			When(minimock.AnyContext, testAccountID, testInitiatorID, testGroupID, testUserID, testRoleID).
			Then(testMember, nil)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() +
			"/members/" + testUserID.String()
		body, _ := json.Marshal(dto.UpdateGroupMemberRequest{RoleID: testRoleID})
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var resp dto.UpdateGroupMemberResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, testUserID, resp.Member.UserID)
		require.Equal(t, testRoleID, resp.Member.RoleID)
		require.Equal(t, "Менеджер", resp.Member.RoleName)
		require.True(t, resp.Member.JoinedAt.Equal(testJoinedAt))
	})

	t.Run("invalid account id", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Return(
			&domain.AuthClaims{UserID: uuid.New(), CurrentAccountID: uuid.New()},
			nil,
		)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		url := "/api/v1/accounts/invalid-uuid/user-groups/" + testGroupID.String() + "/members/" + testUserID.String()
		req := httptest.NewRequest(http.MethodPut, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Return(
			&domain.AuthClaims{UserID: uuid.New(), CurrentAccountID: uuid.New()},
			nil,
		)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() +
			"/members/" + testUserID.String()
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("role from another account is not found", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)

		tx := saga_mocks.NewBobTransactionMock(mc)
		tx.RollbackMock.Expect(minimock.AnyContext).Return(nil)
		repo := saga_mocks.NewTransactableMock(mc)
		repo.WithTxMock.When(minimock.AnyContext).Then(tx, nil)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.GroupMember.UpdateRoleMock.
			When(minimock.AnyContext, testAccountID, testInitiatorID, testGroupID, testUserID, testRoleID).
			Then(domain.GroupMemberDetails{}, service.ErrNotFound)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() +
			"/members/" + testUserID.String()
		body, _ := json.Marshal(dto.UpdateGroupMemberRequest{RoleID: testRoleID})
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("forbidden", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)

		tx := saga_mocks.NewBobTransactionMock(mc)
		tx.RollbackMock.Expect(minimock.AnyContext).Return(nil)
		repo := saga_mocks.NewTransactableMock(mc)
		repo.WithTxMock.When(minimock.AnyContext).Then(tx, nil)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.GroupMember.UpdateRoleMock.
			When(minimock.AnyContext, testAccountID, testInitiatorID, testGroupID, testUserID, testRoleID).
			Then(domain.GroupMemberDetails{}, service.ErrForbidden)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() +
			"/members/" + testUserID.String()
		body, _ := json.Marshal(dto.UpdateGroupMemberRequest{RoleID: testRoleID})
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)

		tx := saga_mocks.NewBobTransactionMock(mc)
		tx.RollbackMock.Expect(minimock.AnyContext).Return(nil)
		repo := saga_mocks.NewTransactableMock(mc)
		repo.WithTxMock.When(minimock.AnyContext).Then(tx, nil)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.GroupMember.UpdateRoleMock.
			When(minimock.AnyContext, testAccountID, testInitiatorID, testGroupID, testUserID, testRoleID).
			Then(domain.GroupMemberDetails{}, errors.New("test error"))

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() +
			"/members/" + testUserID.String()
		body, _ := json.Marshal(dto.UpdateGroupMemberRequest{RoleID: testRoleID})
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
