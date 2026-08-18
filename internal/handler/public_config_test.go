package handler_test

import (
	"testing"
	"time"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/handler"
	"vilib-api/internal/service"

	"github.com/stretchr/testify/require"
)

// TestBuildPublicConfig проверяет сборку публичного конфига фронтенда (§5.2 контракта Э2,
// П-8) из config.Config: значения переносятся дословно, MIME-тип и минимальная длина пароля —
// фиксированные константы, не настраиваемые через конфигурацию.
func TestBuildPublicConfig(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Video: config.VideoConfig{
			MaxUploadSizeBytes:       4 << 30,
			Profiles:                 []string{"360p", "720p", "1080p"},
			HLSURLTTL:                3600 * 1e9,
			WatchHeartbeatInterval:   10 * time.Second,
			WatchCompletionThreshold: 0.95,
		},
	}

	got := handler.BuildPublicConfig(cfg)

	require.Equal(t, int64(4<<30), got.MaxUploadSizeBytes)
	require.Equal(t, []string{"video/*"}, got.AllowedContentTypes)
	require.Equal(t, int64(domain.VideoUploadURLTTL.Seconds()), got.UploadURLTTLSeconds)
	require.Equal(t, int64(3600), got.HLSURLTTLSeconds)
	require.Equal(t, []string{"360p", "720p", "1080p"}, got.Profiles)
	require.Equal(t, int64(service.DefaultJWTExpireDuration.Seconds()), got.TokenTTLSeconds)
	require.Equal(t, service.PasswordMinLength, got.PasswordMinLength)
	require.Equal(t, int64(10), got.HeartbeatSeconds)
	require.InDelta(t, 0.95, got.CompletionThreshold, 0)
}
