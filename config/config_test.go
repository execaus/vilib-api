package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	"vilib-api/config"

	"github.com/stretchr/testify/require"
)

// TestLoadConfig_UsesDefaultsWhenFileIsAbsent проверяет, что при отсутствии файла конфигурации
// LoadConfig не падает и заполняет конфигурацию дефолтными значениями.
func TestLoadConfig_UsesDefaultsWhenFileIsAbsent(t *testing.T) {
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "does-not-exist.yaml"))

	cfg, err := config.LoadConfig()
	require.NoError(t, err)

	require.Equal(t, "8080", cfg.Server.Port)
	require.Equal(t, time.Hour, cfg.Auth.PasswordResetTTL)
	require.Equal(t, "app", cfg.Database.Schema)
	require.Equal(t, "disable", cfg.Database.SSLMode)
	require.Equal(t, "us-east-1", cfg.S3.Region)
	require.Equal(t, "vilib", cfg.S3.Bucket)
	require.True(t, cfg.S3.UsePathStyle)
	require.Equal(t, "video.original-uploaded", cfg.Kafka.TopicOriginalUploaded)
	require.Equal(t, "video.processing-events", cfg.Kafka.TopicProcessingEvents)
	require.Equal(t, "vilib-api", cfg.Kafka.ConsumerGroup)
	require.Equal(t, 500*time.Millisecond, cfg.Kafka.OutboxPollInterval)
	require.Equal(t, 100, cfg.Kafka.OutboxBatchSize)
	require.Equal(t, int64(4294967296), cfg.Video.MaxUploadSizeBytes)
	require.Equal(t, []string{"360p", "720p", "1080p"}, cfg.Video.Profiles)
	require.Equal(t, 3, cfg.Video.MaxProcessingAttempts)
	require.Equal(t, 2*time.Hour, cfg.Video.UploadTimeout)
	require.Equal(t, time.Hour, cfg.Video.QueuedTimeout)
	require.Equal(t, 3*time.Hour, cfg.Video.ProcessingTimeout)
	require.Equal(t, time.Minute, cfg.Video.WatchdogInterval)
	require.Equal(t, time.Hour, cfg.Video.HLSURLTTL)
	require.Equal(t, time.Hour, cfg.Video.HLSSegmentTTL)
	require.InDelta(t, 0.95, cfg.Video.WatchCompletionThreshold, 0)
	require.Equal(t, 10*time.Second, cfg.Video.WatchHeartbeatInterval)
	require.Equal(t, 30*24*time.Hour, cfg.Video.WatchSessionRetention)
}

// TestLoadConfig_EnvironmentOverridesDefaults проверяет, что переменные окружения переопределяют
// дефолтные значения, включая списки через запятую и продолжительности.
func TestLoadConfig_EnvironmentOverridesDefaults(t *testing.T) {
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("DATABASE_HOST", "db.internal")
	t.Setenv("DATABASE_SCHEMA", "custom")
	t.Setenv("DATABASE_SSLMODE", "require")
	t.Setenv("AUTH_KEY", "secret")
	t.Setenv("AUTH_PASSWORD_RESET_TTL", "2h")
	t.Setenv("FRONTEND_ORIGIN", "http://localhost:5173")
	t.Setenv("S3_ENDPOINT", "http://minio:9000")
	t.Setenv("S3_PUBLIC_ENDPOINT", "http://localhost:9000")
	t.Setenv("KAFKA_BROKERS", "broker1:9092,broker2:9092")
	t.Setenv("KAFKA_OUTBOX_POLL_INTERVAL", "750ms")
	t.Setenv("VIDEO_PROFILES", "360p,720p")
	t.Setenv("VIDEO_UPLOAD_TIMEOUT", "45m")
	t.Setenv("VIDEO_WATCH_COMPLETION_THRESHOLD", "0.8")
	t.Setenv("VIDEO_WATCH_HEARTBEAT_INTERVAL", "15s")

	cfg, err := config.LoadConfig()
	require.NoError(t, err)

	require.Equal(t, "9090", cfg.Server.Port)
	require.Equal(t, "db.internal", cfg.Database.Host)
	require.Equal(t, "custom", cfg.Database.Schema)
	require.Equal(t, "require", cfg.Database.SSLMode)
	require.Equal(t, "secret", cfg.Auth.Key)
	require.Equal(t, 2*time.Hour, cfg.Auth.PasswordResetTTL)
	require.Equal(t, "http://localhost:5173", cfg.Frontend.Origin)
	require.Equal(t, "http://minio:9000", cfg.S3.Endpoint)
	require.Equal(t, "http://localhost:9000", cfg.S3.PublicEndpoint)
	require.Equal(t, []string{"broker1:9092", "broker2:9092"}, cfg.Kafka.Brokers)
	require.Equal(t, 750*time.Millisecond, cfg.Kafka.OutboxPollInterval)
	require.Equal(t, []string{"360p", "720p"}, cfg.Video.Profiles)
	require.Equal(t, 45*time.Minute, cfg.Video.UploadTimeout)
	require.InDelta(t, 0.8, cfg.Video.WatchCompletionThreshold, 0)
	require.Equal(t, 15*time.Second, cfg.Video.WatchHeartbeatInterval)
}

// TestLoadConfig_ReadsValuesFromFile проверяет чтение значений из YAML-файла конфигурации.
func TestLoadConfig_ReadsValuesFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `
server:
  port: "7000"
database:
  host: "localhost"
  schema: "app"
s3:
  bucket: "from-file"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	t.Setenv("CONFIG_PATH", path)

	cfg, err := config.LoadConfig()
	require.NoError(t, err)

	require.Equal(t, "7000", cfg.Server.Port)
	require.Equal(t, "localhost", cfg.Database.Host)
	require.Equal(t, "from-file", cfg.S3.Bucket)
}

// TestLoadConfig_ReturnsErrorOnMalformedFile проверяет, что при ошибке разбора файла
// LoadConfig возвращает ошибку, а не пустой конфиг.
func TestLoadConfig_ReturnsErrorOnMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server:\n  port: [broken"), 0o600))
	t.Setenv("CONFIG_PATH", path)

	_, err := config.LoadConfig()
	require.Error(t, err)
}

// TestConfig_Validate проверяет, что Validate находит каждое из обязательных полей отдельно.
func TestConfig_Validate(t *testing.T) {
	t.Parallel()

	validConfig := func() config.Config {
		return config.Config{
			Database: config.DatabaseConfig{
				Host: "h", Port: "p", User: "u", Name: "n", Schema: "s",
			},
			Auth:     config.AuthConfig{Key: "key", PasswordResetTTL: time.Hour},
			Frontend: config.FrontendConfig{Origin: "http://localhost:5173"},
			S3: config.S3Config{
				Endpoint: "e", PublicEndpoint: "pe", AccessKeyID: "ak", SecretAccessKey: "sk", Bucket: "b",
			},
			Kafka: config.KafkaConfig{
				Brokers:               []string{"b1"},
				TopicOriginalUploaded: "t1",
				TopicProcessingEvents: "t2",
				ConsumerGroup:         "g",
			},
			Video: config.VideoConfig{
				Profiles:                 []string{"360p"},
				MaxProcessingAttempts:    1,
				UploadTimeout:            time.Hour,
				QueuedTimeout:            time.Hour,
				ProcessingTimeout:        time.Hour,
				WatchdogInterval:         time.Hour,
				HLSURLTTL:                time.Hour,
				HLSSegmentTTL:            time.Hour,
				WatchCompletionThreshold: 0.95,
				WatchHeartbeatInterval:   10 * time.Second,
				WatchSessionRetention:    30 * 24 * time.Hour,
			},
		}
	}

	t.Run("valid config passes", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, validConfig().Validate())
	})

	tests := []struct {
		name    string
		mutate  func(cfg *config.Config)
		wantErr string
	}{
		{
			name:    "missing database host",
			mutate:  func(cfg *config.Config) { cfg.Database.Host = "" },
			wantErr: "database.host",
		},
		{
			name:    "missing auth key",
			mutate:  func(cfg *config.Config) { cfg.Auth.Key = "" },
			wantErr: "auth.key",
		},
		{
			name:    "zero auth password reset ttl",
			mutate:  func(cfg *config.Config) { cfg.Auth.PasswordResetTTL = 0 },
			wantErr: "auth.password_reset_ttl",
		},
		{
			name:    "missing frontend origin",
			mutate:  func(cfg *config.Config) { cfg.Frontend.Origin = "" },
			wantErr: "frontend.origin",
		},
		{
			name:    "missing s3 endpoint",
			mutate:  func(cfg *config.Config) { cfg.S3.Endpoint = "" },
			wantErr: "s3.endpoint",
		},
		{
			name:    "missing s3 public endpoint",
			mutate:  func(cfg *config.Config) { cfg.S3.PublicEndpoint = "" },
			wantErr: "s3.public_endpoint",
		},
		{
			name:    "missing kafka brokers",
			mutate:  func(cfg *config.Config) { cfg.Kafka.Brokers = nil },
			wantErr: "kafka.brokers",
		},
		{
			name:    "missing kafka topic",
			mutate:  func(cfg *config.Config) { cfg.Kafka.TopicOriginalUploaded = "" },
			wantErr: "kafka.topic_original_uploaded",
		},
		{
			name:    "empty video profiles",
			mutate:  func(cfg *config.Config) { cfg.Video.Profiles = nil },
			wantErr: "video.profiles",
		},
		{
			name:    "zero max processing attempts",
			mutate:  func(cfg *config.Config) { cfg.Video.MaxProcessingAttempts = 0 },
			wantErr: "video.max_processing_attempts",
		},
		{
			name:    "zero upload timeout",
			mutate:  func(cfg *config.Config) { cfg.Video.UploadTimeout = 0 },
			wantErr: "video.upload_timeout",
		},
		{
			name:    "zero watch completion threshold",
			mutate:  func(cfg *config.Config) { cfg.Video.WatchCompletionThreshold = 0 },
			wantErr: "video.watch_completion_threshold",
		},
		{
			name:    "watch completion threshold above one",
			mutate:  func(cfg *config.Config) { cfg.Video.WatchCompletionThreshold = 1.01 },
			wantErr: "video.watch_completion_threshold",
		},
		{
			name:    "zero watch heartbeat interval",
			mutate:  func(cfg *config.Config) { cfg.Video.WatchHeartbeatInterval = 0 },
			wantErr: "video.watch_heartbeat_interval",
		},
		{
			name:    "zero watch session retention",
			mutate:  func(cfg *config.Config) { cfg.Video.WatchSessionRetention = 0 },
			wantErr: "video.watch_session_retention",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := validConfig()
			tt.mutate(&cfg)

			err := cfg.Validate()
			require.Error(t, err)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
