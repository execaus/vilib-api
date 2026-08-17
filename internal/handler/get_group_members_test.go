package handler_test

import (
	"encoding/json"
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

func TestHandler_GetGroupMembers(t *testing.T) {
	var (
		testAccountID   = uuid.New()
		testGroupID     = uuid.New()
		testInitiatorID = uuid.New()
		testToken       = "valid-token"
		testUserID      = uuid.New()
		testRoleID      = uuid.New()
		testJoinedAt    = time.Now().UTC().Truncate(time.Second)
		testMembers     = []domain.GroupMemberDetails{
			{
				UserID:         testUserID,
				Name:           "Иван",
				Surname:        "Иванов",
				Email:          "ivan@example.com",
				RoleID:         testRoleID,
				RoleName:       "Менеджер",
				PermissionMask: domain.SetBits(0, domain.GroupPermissionManageMembers),
				JoinedAt:       testJoinedAt,
			},
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
		svcMock.GroupMember.ListByGroupMock.When(minimock.AnyContext, testAccountID, testInitiatorID, testGroupID).
			Then(testMembers, nil)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() + "/members"
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var resp dto.GetGroupMembersResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Len(t, resp.Members, 1)
		require.Equal(t, testUserID, resp.Members[0].UserID)
		require.Equal(t, "Иван", resp.Members[0].Name)
		require.Equal(t, "Иванов", resp.Members[0].Surname)
		require.Equal(t, "ivan@example.com", resp.Members[0].Email)
		require.Equal(t, testRoleID, resp.Members[0].RoleID)
		require.Equal(t, "Менеджер", resp.Members[0].RoleName)
		require.True(t, resp.Members[0].JoinedAt.Equal(testJoinedAt))
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
		svcMock.GroupMember.ListByGroupMock.When(minimock.AnyContext, testAccountID, testInitiatorID, testGroupID).
			Then(nil, service.ErrForbidden)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() + "/members"
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("group not found", func(t *testing.T) {
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
		svcMock.GroupMember.ListByGroupMock.When(minimock.AnyContext, testAccountID, testInitiatorID, testGroupID).
			Then(nil, service.ErrNotFound)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() + "/members"
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid account id", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Return(
			&domain.AuthClaims{UserID: uuid.New(), CurrentAccountID: uuid.New()},
			nil,
		)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		url := "/api/v1/accounts/invalid-uuid/user-groups/" + testGroupID.String() + "/members"
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
