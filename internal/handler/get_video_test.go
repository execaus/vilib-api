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

func TestHandler_GetVideo(t *testing.T) {
	var (
		testAccountID = uuid.New()
		testGroupID   = uuid.New()
		testVideoID   = uuid.New()
		testUserID    = uuid.New()
		testToken     = "valid-token"
		testHLSToken  = "hls-token"
		testExpiresAt = time.Now().Add(time.Hour).Truncate(time.Second)
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
		"/video/" + testVideoID.String()

	t.Run("hls kind returns master playlist url with token", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupCommitTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testUserID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.Video.GetMock.
			When(minimock.AnyContext, testAccountID, testGroupID, testUserID, testVideoID, false).
			Then(domain.VideoAccess{
				Kind:      domain.VideoAccessKindHLS,
				HLSToken:  testHLSToken,
				ExpiresAt: testExpiresAt,
				Video:     domain.Video{ID: testVideoID, Status: domain.VideoStatusReady},
				Profiles:  []string{"360p", "720p"},
			}, nil)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)
		req.Host = "vilib.example.com"

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response dto.GetVideoResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Equal(t, "hls", response.Kind)
		require.Contains(t, response.URL, "vilib.example.com")
		masterPath := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() +
			"/video/" + testVideoID.String() + "/hls/master.m3u8"
		require.Contains(t, response.URL, masterPath)
		require.Contains(t, response.URL, "token="+testHLSToken)
		require.Equal(t, uint(domain.VideoStatusReady), response.Status)
		require.Equal(t, "ready", response.StatusName)
		require.Equal(t, []string{"360p", "720p"}, response.Profiles)
	})

	t.Run("original kind returns presigned url as is", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupCommitTx(mc)

		testOriginalURL := domain.PreflightURL("https://example.com/original")

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testUserID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.Video.GetMock.
			When(minimock.AnyContext, testAccountID, testGroupID, testUserID, testVideoID, true).
			Then(domain.VideoAccess{
				Kind:      domain.VideoAccessKindOriginal,
				URL:       testOriginalURL,
				ExpiresAt: testExpiresAt,
				Video:     domain.Video{ID: testVideoID, Status: domain.VideoStatusCompressing},
			}, nil)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		req := httptest.NewRequest(http.MethodGet, url+"?is_prefer_original=true", nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response dto.GetVideoResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Equal(t, "original", response.Kind)
		require.Equal(t, string(testOriginalURL), response.URL)
		require.Equal(t, "compressing", response.StatusName)
		require.Empty(t, response.Profiles)
	})

	t.Run("conflict when video is not available", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testUserID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.Video.GetMock.
			When(minimock.AnyContext, testAccountID, testGroupID, testUserID, testVideoID, false).
			Then(domain.VideoAccess{}, service.NewConflictError("video is not available"))

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("forbidden", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testUserID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.Video.GetMock.
			When(minimock.AnyContext, testAccountID, testGroupID, testUserID, testVideoID, false).
			Then(domain.VideoAccess{}, service.ErrForbidden)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("unauthorized without token", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		req := httptest.NewRequest(http.MethodGet, url, nil)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid video id", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Return(
			&domain.AuthClaims{UserID: uuid.New(), CurrentAccountID: uuid.New()},
			nil,
		)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		invalidURL := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() +
			"/video/invalid-uuid"
		req := httptest.NewRequest(http.MethodGet, invalidURL, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
