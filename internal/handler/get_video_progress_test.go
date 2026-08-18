package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/handler"
	"vilib-api/internal/repository"
	"vilib-api/internal/saga"
	"vilib-api/internal/saga/saga_mocks"
	"vilib-api/internal/service"
	"vilib-api/testutil"

	"github.com/gojuno/minimock/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestHandler_GetVideoProgress(t *testing.T) {
	var (
		testAccountID   = uuid.New()
		testGroupID     = uuid.New()
		testVideoID     = uuid.New()
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

	url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() +
		"/video/" + testVideoID.String() + "/progress"

	t.Run("success with no progress returns zero state", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupCommitTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.WatchProgress.GetMock.
			When(minimock.AnyContext, testAccountID, testGroupID, testInitiatorID, testVideoID).
			Then(domain.WatchState{}, nil)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response dto.WatchProgressResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Zero(t, response.CoveredMs)
		require.False(t, response.Accepted)
	})

	t.Run("unauthorized without token", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		req := httptest.NewRequest(http.MethodGet, url, nil)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("forbidden", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.WatchProgress.GetMock.
			When(minimock.AnyContext, testAccountID, testGroupID, testInitiatorID, testVideoID).
			Then(domain.WatchState{}, service.ErrForbidden)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("not found", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.WatchProgress.GetMock.
			When(minimock.AnyContext, testAccountID, testGroupID, testInitiatorID, testVideoID).
			Then(domain.WatchState{}, repository.ErrNotFound)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid video id", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Return(
			&domain.AuthClaims{UserID: uuid.New(), CurrentAccountID: uuid.New()}, nil,
		)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		badURL := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() +
			"/video/invalid-uuid/progress"
		req := httptest.NewRequest(http.MethodGet, badURL, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
