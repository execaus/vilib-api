package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"vilib-api/internal/dto"
	"vilib-api/internal/handler"
	"vilib-api/internal/saga"
	"vilib-api/internal/saga/saga_mocks"
	"vilib-api/testutil"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/require"
)

// TestHandler_GetConfig проверяет ручку GET /api/v1/config (§5.2 контракта Э2, П-8): работает
// без авторизации, отдаёт собранный при старте конфиг и заголовок Cache-Control.
func TestHandler_GetConfig(t *testing.T) {
	t.Parallel()

	testConfig := dto.ConfigResponse{
		MaxUploadSizeBytes:  4 << 30,
		AllowedContentTypes: []string{"video/*"},
		UploadURLTTLSeconds: 3600,
		HLSURLTTLSeconds:    3600,
		Profiles:            []string{"360p", "720p"},
		TokenTTLSeconds:     86400,
		PasswordMinLength:   8,
	}

	mc := minimock.NewController(t)

	repo := saga_mocks.NewTransactableMock(mc)
	svcMock := testutil.NewHandlerTestServiceMock(mc)

	h := handler.NewHandler(
		saga.NewSagaRunner(svcMock.ToService(), repo),
		handler.Deps{Auth: svcMock.Auth, PublicConfig: testConfig},
	)
	router := h.GetRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "public, max-age=60", w.Header().Get("Cache-Control"))

	var response dto.ConfigResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, testConfig, response)
}
