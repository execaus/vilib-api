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

func TestHandler_RenameVideo(t *testing.T) {
	var (
		testAccountID   = uuid.New()
		testGroupID     = uuid.New()
		testVideoID     = uuid.New()
		testInitiatorID = uuid.New()
		testToken       = "valid-token"
		testNewName     = "updated name"
		testAuthorID    = uuid.New()
		testVideoItem   = domain.VideoListItem{
			Video: domain.Video{
				ID:      testVideoID,
				GroupID: testGroupID,
				Name:    testNewName,
				Author:  testAuthorID,
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

	t.Run("success", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupCommitTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID:           testInitiatorID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.Video.RenameMock.When(minimock.AnyContext, testAccountID, testGroupID, testInitiatorID, testVideoID, testNewName).
			Then(testVideoItem, nil)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() + "/video/" + testVideoID.String()
		body, _ := json.Marshal(dto.RenameVideoRequest{Name: testNewName})
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response dto.RenameVideoResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Equal(t, dto.VideoAuthor{ID: testAuthorID, Name: "Ivan", Surname: "Ivanov"}, response.Video.Author)
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
		svcMock.Video.RenameMock.When(minimock.AnyContext, testAccountID, testGroupID, testInitiatorID, testVideoID, testNewName).
			Then(domain.VideoListItem{}, service.ErrForbidden)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() + "/video/" + testVideoID.String()
		body, _ := json.Marshal(dto.RenameVideoRequest{Name: testNewName})
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusForbidden, w.Code)
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
		svcMock.Video.RenameMock.When(minimock.AnyContext, testAccountID, testGroupID, testInitiatorID, testVideoID, testNewName).
			Then(domain.VideoListItem{}, service.ErrNotFound)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() + "/video/" + testVideoID.String()
		body, _ := json.Marshal(dto.RenameVideoRequest{Name: testNewName})
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Return(
			&domain.AuthClaims{UserID: uuid.New(), CurrentAccountID: uuid.New()},
			nil,
		)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() + "/video/" + testVideoID.String()
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
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

		url := "/api/v1/accounts/invalid-uuid/user-groups/" + testGroupID.String() + "/video/" + testVideoID.String()
		body, _ := json.Marshal(dto.RenameVideoRequest{Name: testNewName})
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
