package handler_test

import (
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

func TestHandler_CompleteVideoUpload(t *testing.T) {
	var (
		testAccountID   = uuid.New()
		testGroupID     = uuid.New()
		testVideoID     = uuid.New()
		testInitiatorID = uuid.New()
		testToken       = "valid-token"
		testAuthorID    = uuid.New()
		testVideoItem   = domain.VideoListItem{
			Video: domain.Video{
				ID: testVideoID, GroupID: testGroupID, Author: testAuthorID, Status: domain.VideoStatusQueued,
			},
			Author: domain.VideoAuthor{ID: testAuthorID, Name: "Ivan", Surname: "Ivanov"},
		}
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
		"/video/" + testVideoID.String() + "/complete"

	t.Run("success", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupCommitTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.Video.CompleteUploadMock.
			When(minimock.AnyContext, testAccountID, testGroupID, testInitiatorID, testVideoID).
			Then(testVideoItem, nil)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		req := httptest.NewRequest(http.MethodPost, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response dto.CompleteVideoUploadResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Equal(t, testVideoID, response.Video.ID)
		require.Equal(t, "queued", response.Video.StatusName)
		require.Equal(t, dto.VideoAuthor{ID: testAuthorID, Name: "Ivan", Surname: "Ivanov"}, response.Video.Author)
	})

	t.Run("not found", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.Video.CompleteUploadMock.
			When(minimock.AnyContext, testAccountID, testGroupID, testInitiatorID, testVideoID).
			Then(domain.VideoListItem{}, service.ErrNotFound)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		req := httptest.NewRequest(http.MethodPost, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("conflict when object not found in storage", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.Video.CompleteUploadMock.
			When(minimock.AnyContext, testAccountID, testGroupID, testInitiatorID, testVideoID).
			Then(domain.VideoListItem{}, service.NewConflictError("object not found in storage"))

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		req := httptest.NewRequest(http.MethodPost, url, nil)
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
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.Video.CompleteUploadMock.
			When(minimock.AnyContext, testAccountID, testGroupID, testInitiatorID, testVideoID).
			Then(domain.VideoListItem{}, service.ErrForbidden)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		req := httptest.NewRequest(http.MethodPost, url, nil)
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

		req := httptest.NewRequest(http.MethodPost, url, nil)

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
			"/video/invalid-uuid/complete"
		req := httptest.NewRequest(http.MethodPost, invalidURL, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
