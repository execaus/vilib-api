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

func TestHandler_UploadVideo(t *testing.T) {
	var (
		testAccountID = uuid.New()
		testGroupID   = uuid.New()
		testUserID    = uuid.New()
		testToken     = "valid-token"
		testURL       = "https://example.com/upload"
	)

	t.Run("success", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		svcMock.Auth.GetClaimsFromTokenMock.Expect("Bearer "+testToken).Return(&domain.AuthClaims{
			UserID:           testUserID,
			CurrentAccountID: testAccountID,
		}, nil)
		svcMock.Video.GetPreflightUploadURLMock.Expect(minimock.AnyContext, testAccountID, testGroupID, testUserID).Return(domain.PreflightURL(testURL), nil)

		router := testutil.SetupTestRouterWithMocks(mc, svcMock, nil)

		url := "/api/v1/accounts/" + testAccountID.String() + "/user-groups/" + testGroupID.String() + "/video"
		req := httptest.NewRequest(http.MethodPost, url, nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})
}
