package handler_test

import (
	"bytes"
	"context"
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

func TestHandler_UpdateAssignment(t *testing.T) {
	var (
		testAccountID    = uuid.New()
		testInitiatorID  = uuid.New()
		testAssignmentID = uuid.New()
		testToken        = "valid-token"
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

	url := "/api/v1/accounts/" + testAccountID.String() + "/assignments/" + testAssignmentID.String()

	body := func(t *testing.T, req dto.UpdateAssignmentRequest) *bytes.Reader {
		t.Helper()

		raw, err := json.Marshal(req)
		require.NoError(t, err)

		return bytes.NewReader(raw)
	}

	dueDays := 10
	daysMode := "days"

	t.Run("success", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupCommitTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID: testInitiatorID, CurrentAccountID: testAccountID,
		}, nil)
		svcMock.Assignment.UpdateDueMock.Set(func(
			_ context.Context, accountID, initiatorID, id uuid.UUID, patch domain.UpdateAssignment,
		) (domain.AssignmentDetails, error) {
			require.Equal(t, testAccountID, accountID)
			require.Equal(t, testInitiatorID, initiatorID)
			require.Equal(t, testAssignmentID, id)
			require.NotNil(t, patch.DueMode)
			require.Equal(t, domain.AssignmentDueModeDays, *patch.DueMode)
			require.NotNil(t, patch.DueDays)
			require.Equal(t, dueDays, *patch.DueDays)

			return domain.AssignmentDetails{Assignment: domain.Assignment{ID: testAssignmentID}}, nil
		})

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		req := httptest.NewRequest(http.MethodPatch, url, body(t, dto.UpdateAssignmentRequest{
			DueMode: &daysMode, DueDays: &dueDays,
		}))
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var resp dto.UpdateAssignmentResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, testAssignmentID, resp.Assignment.ID)
	})

	t.Run("unknown due mode is bad request", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Return(&domain.AuthClaims{
			UserID: testInitiatorID, CurrentAccountID: testAccountID,
		}, nil)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		unknownMode := "forever"
		req := httptest.NewRequest(http.MethodPatch, url, body(t, dto.UpdateAssignmentRequest{
			DueMode: &unknownMode,
		}))
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("cancelled assignment is conflict", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		repo := setupRollbackTx(mc)

		svcMock.Auth.GetClaimsFromTokenMock.When("Bearer "+testToken).Then(&domain.AuthClaims{
			UserID: testInitiatorID, CurrentAccountID: testAccountID,
		}, nil)
		svcMock.Assignment.UpdateDueMock.Return(domain.AssignmentDetails{}, service.ErrAssignmentCancelled)

		h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
		router := h.GetRouter()

		req := httptest.NewRequest(http.MethodPatch, url, body(t, dto.UpdateAssignmentRequest{
			DueMode: &daysMode, DueDays: &dueDays,
		}))
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("unauthorized without token", func(t *testing.T) {
		mc := minimock.NewController(t)

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		req := httptest.NewRequest(http.MethodPatch, url, body(t, dto.UpdateAssignmentRequest{}))

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
