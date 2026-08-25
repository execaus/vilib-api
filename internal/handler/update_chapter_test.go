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

func TestHandler_UpdateChapter(t *testing.T) {
	var (
		testAccountID   = uuid.New()
		testGroupID     = uuid.New()
		testVideoID     = uuid.New()
		testChapterID   = uuid.New()
		testInitiatorID = uuid.New()
		testToken       = "valid-token"
		testNewName     = "Итоговое название"
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
		"/video/" + testVideoID.String() + "/chapters/" + testChapterID.String()

	t.Run("success", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupCommitTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		bound := domain.ChapterBound{
			Chapter: domain.Chapter{ID: testChapterID, VideoID: testVideoID, Name: testNewName, StartMs: 1000},
			EndMs:   10000,
		}
		svcMock.Chapter.UpdateMock.
			When(
				minimock.AnyContext, testAccountID, testGroupID, testInitiatorID, testVideoID, testChapterID,
				domain.ChapterPatch{Name: &testNewName},
			).
			Then(bound, nil)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		body, _ := json.Marshal(dto.UpdateChapterRequest{Name: &testNewName})
		req := httptest.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response dto.ChapterResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Equal(t, testNewName, response.Chapter.Name)
	})

	t.Run("duplicate start_ms returns conflict", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		newStart := int64(5000)
		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.Chapter.UpdateMock.
			When(
				minimock.AnyContext, testAccountID, testGroupID, testInitiatorID, testVideoID, testChapterID,
				domain.ChapterPatch{StartMs: &newStart},
			).
			Then(domain.ChapterBound{}, service.ErrChapterStartTaken)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		body, _ := json.Marshal(dto.UpdateChapterRequest{StartMs: &newStart})
		req := httptest.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusConflict, w.Code)

		var response dto.ErrorMessage
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Equal(t, "conflict.chapter_start_taken", response.Code)
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
		svcMock.Chapter.UpdateMock.
			When(
				minimock.AnyContext, testAccountID, testGroupID, testInitiatorID, testVideoID, testChapterID,
				domain.ChapterPatch{Name: &testNewName},
			).
			Then(domain.ChapterBound{}, service.ErrNotFound)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		body, _ := json.Marshal(dto.UpdateChapterRequest{Name: &testNewName})
		req := httptest.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid chapter id", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Return(
			&domain.AuthClaims{UserID: uuid.New(), CurrentAccountID: uuid.New()},
			nil,
		)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		badURL := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() +
			"/video/" + testVideoID.String() + "/chapters/invalid-uuid"
		body, _ := json.Marshal(dto.UpdateChapterRequest{Name: &testNewName})
		req := httptest.NewRequest(http.MethodPatch, badURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
