package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"
	"vilib-api/testutil"

	"github.com/gojuno/minimock/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestHandler_RequireAuthMiddleware проверяет разбор заголовка Authorization: отсутствие
// заголовка, "Bearer <jwt>", голый "<jwt>" (обратная совместимость с e2e), просроченный и
// битый токен (§2.1 дизайна эпика).
func TestHandler_RequireAuthMiddleware(t *testing.T) {
	var (
		testAccountID   = uuid.New()
		testInitiatorID = uuid.New()
		testToken       = "valid-token"
	)

	url := "/api/v1/accounts/" + testAccountID.String() + "/roles"

	tests := []struct {
		name           string
		header         string
		setupMocks     func(auth *testutil.HandlerTestServiceMock)
		wantStatus     int
		wantCode       string
		expectBusiness bool
	}{
		{
			name:       "missing header returns unauthorized",
			header:     "",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:   "bearer prefixed token passes to handler",
			header: "Bearer " + testToken,
			setupMocks: func(m *testutil.HandlerTestServiceMock) {
				m.Auth.GetClaimsFromTokenMock.Expect("Bearer "+testToken).Return(&domain.AuthClaims{
					UserID:           testInitiatorID,
					CurrentAccountID: testAccountID,
				}, nil)
			},
			wantStatus:     http.StatusOK,
			expectBusiness: true,
		},
		{
			name:   "bare token without bearer prefix passes to handler",
			header: testToken,
			setupMocks: func(m *testutil.HandlerTestServiceMock) {
				m.Auth.GetClaimsFromTokenMock.Expect(testToken).Return(&domain.AuthClaims{
					UserID:           testInitiatorID,
					CurrentAccountID: testAccountID,
				}, nil)
			},
			wantStatus:     http.StatusOK,
			expectBusiness: true,
		},
		{
			name:   "expired token returns token_expired",
			header: "Bearer " + testToken,
			setupMocks: func(m *testutil.HandlerTestServiceMock) {
				m.Auth.GetClaimsFromTokenMock.Expect("Bearer "+testToken).Return(nil, service.ErrTokenExpired)
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "token_expired",
		},
		{
			name:   "malformed token returns token_invalid",
			header: "Bearer " + testToken,
			setupMocks: func(m *testutil.HandlerTestServiceMock) {
				m.Auth.GetClaimsFromTokenMock.Expect("Bearer "+testToken).Return(nil, service.ErrTokenInvalid)
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "token_invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := minimock.NewController(t)

			svcMock := testutil.NewHandlerTestServiceMock(mc)
			if tt.setupMocks != nil {
				tt.setupMocks(svcMock)
			}
			if tt.expectBusiness {
				svcMock.AccountRole.GetAllMock.
					Expect(minimock.AnyContext, testInitiatorID, testAccountID).
					Return(nil, nil)
			}

			router := testutil.SetupTestRouterWithoutTx(mc, svcMock)
			if tt.expectBusiness {
				router = testutil.SetupTestRouterWithMocks(mc, svcMock, nil)
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, tt.wantStatus, w.Code)

			if tt.wantCode != "" {
				var body dto.ErrorMessage
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				require.Equal(t, tt.wantCode, body.Code)
			}
		})
	}
}
