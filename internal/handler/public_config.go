package handler

import (
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"
)

// BuildPublicConfig собирает публичный конфиг фронтенда (§5.2 контракта Э2, П-8) из
// конфигурации приложения — вызывается один раз при старте (cmd/main.go) и передаётся в
// Deps.PublicConfig, чтобы ручка GetConfig не зависела от саги/БД. AllowedContentTypes —
// фиксированное значение: сервер уже проверяет только префикс "video/" (videoContentTypePrefix
// сервиса), детализация до конкретных MIME-типов не требуется.
func BuildPublicConfig(cfg config.Config) dto.ConfigResponse {
	return dto.ConfigResponse{
		MaxUploadSizeBytes:  cfg.Video.MaxUploadSizeBytes,
		AllowedContentTypes: []string{"video/*"},
		UploadURLTTLSeconds: int64(domain.VideoUploadURLTTL.Seconds()),
		HLSURLTTLSeconds:    int64(cfg.Video.HLSURLTTL.Seconds()),
		Profiles:            cfg.Video.Profiles,
		TokenTTLSeconds:     int64(service.DefaultJWTExpireDuration.Seconds()),
		PasswordMinLength:   service.PasswordMinLength,
		HeartbeatSeconds:    int64(cfg.Video.WatchHeartbeatInterval.Seconds()),
		CompletionThreshold: cfg.Video.WatchCompletionThreshold,
	}
}
