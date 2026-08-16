package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vilib-api/internal/dto"
	"vilib-api/testutil"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/require"
)

func TestHandler_Health(t *testing.T) {
	t.Run("returns ok without authorization", func(t *testing.T) {
		mc := minimock.NewController(t)
		defer mc.Finish()

		svcMock := testutil.NewHandlerTestServiceMock(mc)
		router := testutil.SetupTestRouterWithoutTx(mc, svcMock)

		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var resp dto.HealthResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, "ok", resp.Status)
	})
}
