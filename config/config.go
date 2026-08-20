package config

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"vilib-api/server"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// configPathEnv — переменная окружения с путём до файла конфигурации.
const configPathEnv = "CONFIG_PATH"

// Config — корневая конфигурация приложения, собирается из файла и переменных окружения.
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Email    EmailConfig    `mapstructure:"email"`
	S3       S3Config       `mapstructure:"s3"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	Video    VideoConfig    `mapstructure:"video"`
	Frontend FrontendConfig `mapstructure:"frontend"`
}

// ServerConfig — параметры HTTP-сервера.
type ServerConfig struct {
	Origin string      `mapstructure:"origin"`
	Port   string      `mapstructure:"port"`
	Mode   server.Mode `mapstructure:"mode"`
}

// DatabaseConfig — параметры подключения к PostgreSQL.
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	Schema   string `mapstructure:"schema"`
	SSLMode  string `mapstructure:"sslmode"`
}

// AuthConfig — параметры выпуска и проверки токенов авторизации.
type AuthConfig struct {
	Key string `mapstructure:"key"`
	// PasswordResetTTL — время жизни токена сброса пароля (§6 дизайна эпика Э2, поправка О-1).
	PasswordResetTTL time.Duration `mapstructure:"password_reset_ttl"`
}

// FrontendConfig — параметры веб-интерфейса, нужные API для построения пользовательских ссылок
// (например, ссылки сброса пароля, §6 дизайна эпика Э2).
type FrontendConfig struct {
	// Origin — базовый адрес веб-интерфейса (протокол, хост, порт — без завершающего слеша).
	Origin string `mapstructure:"origin"`
}

// EmailConfig — параметры SMTP-сервера для отправки писем.
type EmailConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
}

// S3Config — параметры подключения к S3-совместимому хранилищу.
type S3Config struct {
	// Endpoint — внутренний адрес хранилища, используется для Head/Get/Delete/List.
	Endpoint string `mapstructure:"endpoint"`
	// PublicEndpoint — адрес, по которому presigned-ссылки откроет клиент; входит в подпись SigV4.
	PublicEndpoint  string `mapstructure:"public_endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	Region          string `mapstructure:"region"`
	Bucket          string `mapstructure:"bucket"`
	// UsePathStyle включает path-style адресацию бакетов (обязательно для MinIO).
	UsePathStyle bool `mapstructure:"use_path_style"`
}

// KafkaConfig — параметры брокера сообщений и релея outbox-событий.
type KafkaConfig struct {
	Brokers               []string `mapstructure:"brokers"`
	TopicOriginalUploaded string   `mapstructure:"topic_original_uploaded"`
	TopicProcessingEvents string   `mapstructure:"topic_processing_events"`
	ConsumerGroup         string   `mapstructure:"consumer_group"`
	// OutboxPollInterval — период опроса таблицы outbox_events релеем.
	OutboxPollInterval time.Duration `mapstructure:"outbox_poll_interval"`
	// OutboxBatchSize — максимальное число событий, публикуемых за один проход релея.
	OutboxBatchSize int `mapstructure:"outbox_batch_size"`
}

// VideoConfig — параметры обработки видео: лимиты, профили, таймауты жизненного цикла.
type VideoConfig struct {
	MaxUploadSizeBytes    int64    `mapstructure:"max_upload_size_bytes"`
	Profiles              []string `mapstructure:"profiles"`
	MaxProcessingAttempts int      `mapstructure:"max_processing_attempts"`
	// UploadTimeout — время на подтверждение загрузки оригинала после выдачи presigned URL.
	UploadTimeout time.Duration `mapstructure:"upload_timeout"`
	// QueuedTimeout — время ожидания взятия видео воркером в очереди.
	QueuedTimeout time.Duration `mapstructure:"queued_timeout"`
	// ProcessingTimeout — время на обработку видео воркером.
	ProcessingTimeout time.Duration `mapstructure:"processing_timeout"`
	// WatchdogInterval — период проверки зависших видео.
	WatchdogInterval time.Duration `mapstructure:"watchdog_interval"`
	// HLSURLTTL — время жизни подписанной ссылки на HLS-манифест.
	HLSURLTTL time.Duration `mapstructure:"hls_url_ttl"`
	// HLSSegmentTTL — время жизни подписанной ссылки на сегмент HLS.
	HLSSegmentTTL time.Duration `mapstructure:"hls_segment_ttl"`
	// WatchCompletionThreshold — доля покрытия видео (0;1], после которой просмотр считается
	// подтверждённым (В-1 решение владельца, §3 дизайна эпика Э3).
	WatchCompletionThreshold float64 `mapstructure:"watch_completion_threshold"`
	// WatchHeartbeatInterval — ожидаемый период отправки heartbeat'ов плеером; используется как
	// базовый интервал для расчёта допустимой длины первого интервала сессии (§3 дизайна эпика Э3).
	WatchHeartbeatInterval time.Duration `mapstructure:"watch_heartbeat_interval"`
	// WatchSessionRetention — срок хранения сессий просмотра: тик watchdog'а удаляет сессии,
	// не обновлявшиеся дольше этого срока (решение О-2 эпика Э3). Сессии нужны только для
	// идемпотентности и антиперемотки живого просмотра, история хранится в watch_progress.
	WatchSessionRetention time.Duration `mapstructure:"watch_session_retention"`
}

// LoadConfig собирает конфигурацию из файла (путь — CONFIG_PATH, по умолчанию config/config.yaml)
// и переменных окружения. Файл необязателен: при его отсутствии используются дефолты и env.
// Любая другая ошибка чтения или разбора конфигурации приводит к возврату ошибки — приложение
// не должно молча стартовать с пустым конфигом.
func LoadConfig() (Config, error) {
	var cfg Config

	v := viper.New()
	setDefaults(v)

	path := os.Getenv(configPathEnv)
	if path == "" {
		path = defaultConfigPath
	}
	v.SetConfigFile(path)

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		var notFoundErr viper.ConfigFileNotFoundError
		if errors.As(err, &notFoundErr) || os.IsNotExist(err) {
			zap.L().Warn("config file not found, using defaults and environment variables", zap.String("path", path))
		} else {
			return cfg, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	if err := v.Unmarshal(&cfg); err != nil {
		return cfg, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

// Validate проверяет, что обязательные для старта приложения поля конфигурации заполнены.
// Возвращает ошибку со списком отсутствующих ключей — приложение не должно молча стартовать
// с неполной конфигурацией.
func (c Config) Validate() error {
	var missing []string

	requiredStrings := map[string]string{
		"database.host":                 c.Database.Host,
		"database.port":                 c.Database.Port,
		"database.user":                 c.Database.User,
		"database.name":                 c.Database.Name,
		"database.schema":               c.Database.Schema,
		"auth.key":                      c.Auth.Key,
		"frontend.origin":               c.Frontend.Origin,
		"s3.endpoint":                   c.S3.Endpoint,
		"s3.public_endpoint":            c.S3.PublicEndpoint,
		"s3.access_key_id":              c.S3.AccessKeyID,
		"s3.secret_access_key":          c.S3.SecretAccessKey,
		"s3.bucket":                     c.S3.Bucket,
		"kafka.topic_original_uploaded": c.Kafka.TopicOriginalUploaded,
		"kafka.topic_processing_events": c.Kafka.TopicProcessingEvents,
		"kafka.consumer_group":          c.Kafka.ConsumerGroup,
	}
	for key, value := range requiredStrings {
		if value == "" {
			missing = append(missing, key)
		}
	}

	if len(c.Kafka.Brokers) == 0 {
		missing = append(missing, "kafka.brokers")
	}
	if len(c.Video.Profiles) == 0 {
		missing = append(missing, "video.profiles")
	}

	if c.Video.MaxProcessingAttempts < 1 {
		missing = append(missing, "video.max_processing_attempts")
	}

	if c.Video.WatchCompletionThreshold <= 0 || c.Video.WatchCompletionThreshold > 1 {
		missing = append(missing, "video.watch_completion_threshold")
	}

	positiveDurations := map[string]time.Duration{
		"auth.password_reset_ttl":        c.Auth.PasswordResetTTL,
		"video.upload_timeout":           c.Video.UploadTimeout,
		"video.queued_timeout":           c.Video.QueuedTimeout,
		"video.processing_timeout":       c.Video.ProcessingTimeout,
		"video.watchdog_interval":        c.Video.WatchdogInterval,
		"video.hls_url_ttl":              c.Video.HLSURLTTL,
		"video.hls_segment_ttl":          c.Video.HLSSegmentTTL,
		"video.watch_heartbeat_interval": c.Video.WatchHeartbeatInterval,
		"video.watch_session_retention":  c.Video.WatchSessionRetention,
	}
	for key, value := range positiveDurations {
		if value <= 0 {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("invalid config, missing or empty values: %s", strings.Join(missing, ", "))
	}

	return nil
}
