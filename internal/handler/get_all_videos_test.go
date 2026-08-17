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

// TestHandler_ListVideos проверяет ручку списка видео группы: успешную отдачу с профилями и
// причиной сбоя по правам инициатора (Э1-Т17, Э1-Т20), отказ в доступе и валидацию path-параметров.
func TestHandler_ListVideos(t *testing.T) {
	var (
		testAccountID   = uuid.New()
		testGroupID     = uuid.New()
		testInitiatorID = uuid.New()
		testToken       = "valid-token"
		testVideoID     = uuid.New()
	)

	// runList поднимает роутер с замоканным Video.GetAll и возвращает ответ на запрос списка видео.
	runList := func(t *testing.T, items []domain.VideoListItem, listErr error) *httptest.ResponseRecorder {
		t.Helper()

		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)

		tx := saga_mocks.NewBobTransactionMock(mc)
		if listErr == nil {
			tx.CommitMock.Expect(minimock.AnyContext).Return(nil)
		} else {
			tx.RollbackMock.Expect(minimock.AnyContext).Return(nil)
		}
		repo := saga_mocks.NewTransactableMock(mc)
		repo.WithTxMock.When(minimock.AnyContext).Then(tx, nil)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.Video.GetAllMock.When(minimock.AnyContext, testAccountID, testGroupID, testInitiatorID).
			Then(items, listErr)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() + "/videos"
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		return w
	}

	t.Run("returns profiles and hides failure without manage video right", func(t *testing.T) {
		items := []domain.VideoListItem{
			{
				Video:        domain.Video{ID: testVideoID, GroupID: testGroupID, Name: "video1"},
				Profiles:     []string{"360p", "720p"},
				HasProcessed: true,
				// Failure не заполнен сервисом (нет ManageVideo у инициатора) — даже если видео failed.
				Failure: nil,
			},
		}

		w := runList(t, items, nil)
		require.Equal(t, http.StatusOK, w.Code)

		var resp dto.GetAllVideosResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Len(t, resp.Videos, 1)
		require.Equal(t, []string{"360p", "720p"}, resp.Videos[0].Profiles)
		require.True(t, resp.Videos[0].HasProcessed)
		require.Nil(t, resp.Videos[0].Failure)
	})

	t.Run("returns failure for initiator with manage video right", func(t *testing.T) {
		items := []domain.VideoListItem{
			{
				Video: domain.Video{ID: testVideoID, GroupID: testGroupID, Name: "video1"},
				Failure: &domain.VideoFailure{
					Class:  domain.VideoFailureClassPermanent,
					Reason: "unsupported codec",
				},
			},
		}

		w := runList(t, items, nil)
		require.Equal(t, http.StatusOK, w.Code)

		var resp dto.GetAllVideosResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Len(t, resp.Videos, 1)
		require.NotNil(t, resp.Videos[0].Failure)
		require.Equal(t, "permanent", resp.Videos[0].Failure.Class)
		require.Equal(t, "unsupported codec", resp.Videos[0].Failure.Reason)
	})

	t.Run("returns empty profiles array, not null, when video has no assets", func(t *testing.T) {
		items := []domain.VideoListItem{
			{Video: domain.Video{ID: testVideoID, GroupID: testGroupID, Name: "video1"}},
		}

		w := runList(t, items, nil)
		require.Equal(t, http.StatusOK, w.Code)
		require.Contains(t, w.Body.String(), `"profiles":[]`)

		var resp dto.GetAllVideosResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Len(t, resp.Videos, 1)
		require.NotNil(t, resp.Videos[0].Profiles)
		require.Empty(t, resp.Videos[0].Profiles)
	})

	t.Run("forbidden", func(t *testing.T) {
		w := runList(t, nil, service.ErrForbidden)
		require.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("invalid account id", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Return(
			&domain.AuthClaims{UserID: uuid.New(), CurrentAccountID: uuid.New()},
			nil,
		)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		url := "/api/v1/accounts/invalid-uuid/user-groups/" + testGroupID.String() + "/videos"
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid group id", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Return(
			&domain.AuthClaims{UserID: uuid.New(), CurrentAccountID: uuid.New()},
			nil,
		)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/invalid-uuid/videos"
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
