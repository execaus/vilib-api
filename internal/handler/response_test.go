package handler_test

import (
	"encoding/json"
	"errors"
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

// TestHandler_SendServiceError_MapsErrorToStatusAndCode проверяет таблицу
// «ошибка сервисного слоя → HTTP-статус + машинный код» (§6.8 ТЗ) через реальный маршрут,
// возвращающий ошибку из мока сервиса.
func TestHandler_SendServiceError_MapsErrorToStatusAndCode(t *testing.T) {
	var (
		testAccountID   = uuid.New()
		testInitiatorID = uuid.New()
		testToken       = "valid-token"
	)

	url := "/api/v1/accounts/" + testAccountID.String() + "/roles"

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantCode   string
		emptyBody  bool
	}{
		{
			name:       "repository not found maps to 404 not_found",
			serviceErr: repository.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "service not found maps to 404 not_found",
			serviceErr: service.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "conflict error without explicit code maps to 409 conflict",
			serviceErr: service.NewConflictError("something conflicts"),
			wantStatus: http.StatusConflict,
			wantCode:   "conflict",
		},
		{
			name:       "conflict error with explicit code is preserved",
			serviceErr: service.NewConflictErrorCode("conflict.account_role_name", "role name exists"),
			wantStatus: http.StatusConflict,
			wantCode:   "conflict.account_role_name",
		},
		{
			name:       "forbidden error maps to 403 forbidden",
			serviceErr: service.ErrForbidden,
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
		},
		{
			name:       "unauthorized error maps to 401 with its code",
			serviceErr: service.ErrUnauthorized,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "unauthorized error with explicit code is preserved",
			serviceErr: service.ErrTokenExpired,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "token_expired",
		},
		{
			name:       "validation error maps to 400 with its code",
			serviceErr: service.NewValidationErrorCode("validation.email", "invalid email"),
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation.email",
		},
		{
			name:       "unknown error maps to 500 with empty body",
			serviceErr: errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			emptyBody:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := minimock.NewController(t)

			svcMock := testutil.NewHandlerTestServiceMock(mc)
			svcMock.Auth.GetClaimsFromTokenMock.Expect("Bearer "+testToken).Return(&domain.AuthClaims{
				UserID:           testInitiatorID,
				CurrentAccountID: testAccountID,
			}, nil)
			svcMock.AccountRole.GetAllMock.
				Expect(minimock.AnyContext, testInitiatorID, testAccountID).
				Return(nil, tt.serviceErr)

			tx := saga_mocks.NewBobTransactionMock(mc)
			tx.RollbackMock.Expect(minimock.AnyContext).Return(nil)
			repo := saga_mocks.NewTransactableMock(mc)
			repo.WithTxMock.Expect(minimock.AnyContext).Return(tx, nil)

			h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo), handler.Deps{Auth: svcMock.Auth})
			router := h.GetRouter()

			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("Authorization", "Bearer "+testToken)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, tt.wantStatus, w.Code)

			if tt.emptyBody {
				require.Empty(t, w.Body.Bytes())
				return
			}

			var body dto.ErrorMessage
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Equal(t, tt.wantCode, body.Code)
			require.NotEmpty(t, body.Message)
		})
	}
}
