package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestHandler_PostVideoProgress(t *testing.T) {
	var (
		testAccountID   = uuid.New()
		testGroupID     = uuid.New()
		testVideoID     = uuid.New()
		testInitiatorID = uuid.New()
		testSessionID   = uuid.New()
		testToken       = "valid-token"
		testDurationMs  = int64(10000)
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

	validRequest := dto.WatchHeartbeatRequest{
		SessionID: testSessionID, Seq: 1, FromMs: 0, ToMs: 5000, PositionMs: 5000, PlaybackRate: 1,
	}

	t.Run("success", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupCommitTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.WatchProgress.HeartbeatMock.
			When(
				minimock.AnyContext,
				testAccountID,
				testGroupID,
				testInitiatorID,
				testVideoID,
				validRequest.ToDomain(),
			).
			Then(domain.WatchState{
				CoveredMs: 5000, CoveragePct: 50, LastPositionMs: 5000,
				DurationMs: &testDurationMs, Accepted: true,
			}, nil)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		body, _ := json.Marshal(validRequest)
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response dto.WatchProgressResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Equal(t, int64(5000), response.CoveredMs)
		require.Equal(t, 50, response.CoveragePct)
		require.True(t, response.Accepted)
		require.Equal(t, testDurationMs, *response.DurationMs)
	})

	t.Run("invalid json", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Return(
			&domain.AuthClaims{UserID: uuid.New(), CurrentAccountID: uuid.New()}, nil,
		)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid account id", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Return(
			&domain.AuthClaims{UserID: uuid.New(), CurrentAccountID: uuid.New()}, nil,
		)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		badURL := "/api/v1/accounts/invalid-uuid/user-groups/" + testGroupID.String() +
			"/video/" + testVideoID.String() + "/progress"
		body, _ := json.Marshal(validRequest)
		req := httptest.NewRequest(http.MethodPost, badURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unauthorized without token", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		body, _ := json.Marshal(validRequest)
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

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
		svcMock.WatchProgress.HeartbeatMock.
			When(
				minimock.AnyContext,
				testAccountID,
				testGroupID,
				testInitiatorID,
				testVideoID,
				validRequest.ToDomain(),
			).
			Then(domain.WatchState{}, service.ErrForbidden)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		body, _ := json.Marshal(validRequest)
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
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
		svcMock.WatchProgress.HeartbeatMock.
			When(
				minimock.AnyContext,
				testAccountID,
				testGroupID,
				testInitiatorID,
				testVideoID,
				validRequest.ToDomain(),
			).
			Then(domain.WatchState{}, repository.ErrNotFound)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		body, _ := json.Marshal(validRequest)
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("conflict video not available", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.WatchProgress.HeartbeatMock.
			When(
				minimock.AnyContext,
				testAccountID,
				testGroupID,
				testInitiatorID,
				testVideoID,
				validRequest.ToDomain(),
			).
			Then(domain.WatchState{}, service.ErrVideoNotAvailable)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		body, _ := json.Marshal(validRequest)
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("validation interval invalid", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.WatchProgress.HeartbeatMock.
			When(
				minimock.AnyContext,
				testAccountID,
				testGroupID,
				testInitiatorID,
				testVideoID,
				validRequest.ToDomain(),
			).
			Then(domain.WatchState{}, service.ErrIntervalInvalid)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		body, _ := json.Marshal(validRequest)
		req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestHandler_PostVideoProgress_ClientTS проверяет, что client_ts корректно проходит через
// биндинг DTO → domain.Heartbeat (используется только в логе, но должен доезжать без потерь).
func TestHandler_PostVideoProgress_ClientTS(t *testing.T) {
	req := dto.WatchHeartbeatRequest{
		SessionID: uuid.New(), Seq: 1, ToMs: 1000, PositionMs: 1000, PlaybackRate: 1,
		ClientTS: time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC),
	}

	got := req.ToDomain()

	require.Equal(t, req.ClientTS, got.ClientTS)
	require.Equal(t, req.SessionID, got.SessionID)
}
