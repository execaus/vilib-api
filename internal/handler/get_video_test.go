package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"vilib-api/internal/domain"
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
		testURL       = "https://example.com/video"
	)

	t.Run("success", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Expect("Bearer "+testToken).Return(&domain.AuthClaims{
			UserID:           testUserID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.Video.GetMock.Expect(minimock.AnyContext, testAccountID, testGroupID, testUserID, testVideoID, false).Return(domain.PreflightURL(testURL), nil)

		router := testutil.SetupTestRouterWithMocks(mc, svcMock, nil)

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() + "/video/" + testVideoID.String()
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("success with prefer original", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Expect("Bearer "+testToken).Return(&domain.AuthClaims{
			UserID:           testUserID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.Video.GetMock.Expect(minimock.AnyContext, testAccountID, testGroupID, testUserID, testVideoID, true).Return(domain.PreflightURL(testURL), nil)

		router := testutil.SetupTestRouterWithMocks(mc, svcMock, nil)

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() + "/video/" + testVideoID.String() + "?is_prefer_original=true"
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})
}
