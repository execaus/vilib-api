package config

import (
	"time"
	"vilib-api/server"

	"github.com/spf13/viper"
)

// defaultConfigPath — путь до файла конфигурации, если не задан CONFIG_PATH.
const defaultConfigPath = "config/config.yaml"

// Дефолты сервера.
const (
	defaultServerPort = "8080"
	defaultServerMode = server.DevelopmentMode
)

// Дефолты подключения к базе данных.
const (
	defaultDatabaseSchema  = "app"
	defaultDatabaseSSLMode = "disable"
)

// Дефолты S3-хранилища.
const (
	defaultS3Region       = "us-east-1"
	defaultS3Bucket       = "vilib"
	defaultS3UsePathStyle = true
)

// Дефолты Kafka.
const (
	defaultKafkaTopicOriginalUploaded = "video.original-uploaded"
	defaultKafkaTopicProcessingEvents = "video.processing-events"
	defaultKafkaConsumerGroup         = "vilib-api"
	defaultKafkaOutboxPollInterval    = 500 * time.Millisecond
	defaultKafkaOutboxBatchSize       = 100
)

// Дефолты обработки видео.
const (
	defaultVideoMaxUploadSizeBytes    int64 = 4294967296
	defaultVideoMaxProcessingAttempts       = 3
	defaultVideoUploadTimeout               = 2 * time.Hour
	defaultVideoQueuedTimeout               = time.Hour
	defaultVideoProcessingTimeout           = 3 * time.Hour
	defaultVideoWatchdogInterval            = time.Minute
	defaultVideoHLSURLTTL                   = time.Hour
	defaultVideoHLSSegmentTTL               = time.Hour
)

// defaultVideoProfiles — дефолтный набор профилей качества видео.
func defaultVideoProfiles() []string {
	return []string{"360p", "720p", "1080p"}
}

// setDefaults регистрирует дефолтное значение для каждого ключа конфигурации.
//
// Это принципиально: viper.AutomaticEnv в связке с Unmarshal видит только ключи,
// уже зарегистрированные через SetDefault/файл/Set — иначе переменная окружения без
// соответствующего дефолта конфигом просто не будет прочитана.
func setDefaults(v *viper.Viper) {
	v.SetDefault("server.origin", "")
	v.SetDefault("server.port", defaultServerPort)
	v.SetDefault("server.mode", defaultServerMode)

	v.SetDefault("database.host", "")
	v.SetDefault("database.port", "")
	v.SetDefault("database.user", "")
	v.SetDefault("database.password", "")
	v.SetDefault("database.name", "")
	v.SetDefault("database.schema", defaultDatabaseSchema)
	v.SetDefault("database.sslmode", defaultDatabaseSSLMode)

	v.SetDefault("auth.key", "")

	v.SetDefault("email.host", "")
	v.SetDefault("email.port", "")
	v.SetDefault("email.username", "")
	v.SetDefault("email.password", "")
	v.SetDefault("email.from", "")

	v.SetDefault("s3.endpoint", "")
	v.SetDefault("s3.public_endpoint", "")
	v.SetDefault("s3.access_key_id", "")
	v.SetDefault("s3.secret_access_key", "")
	v.SetDefault("s3.region", defaultS3Region)
	v.SetDefault("s3.bucket", defaultS3Bucket)
	v.SetDefault("s3.use_path_style", defaultS3UsePathStyle)

	v.SetDefault("kafka.brokers", []string{})
	v.SetDefault("kafka.topic_original_uploaded", defaultKafkaTopicOriginalUploaded)
	v.SetDefault("kafka.topic_processing_events", defaultKafkaTopicProcessingEvents)
	v.SetDefault("kafka.consumer_group", defaultKafkaConsumerGroup)
	v.SetDefault("kafka.outbox_poll_interval", defaultKafkaOutboxPollInterval)
	v.SetDefault("kafka.outbox_batch_size", defaultKafkaOutboxBatchSize)

	v.SetDefault("video.max_upload_size_bytes", defaultVideoMaxUploadSizeBytes)
	v.SetDefault("video.profiles", defaultVideoProfiles())
	v.SetDefault("video.max_processing_attempts", defaultVideoMaxProcessingAttempts)
	v.SetDefault("video.upload_timeout", defaultVideoUploadTimeout)
	v.SetDefault("video.queued_timeout", defaultVideoQueuedTimeout)
	v.SetDefault("video.processing_timeout", defaultVideoProcessingTimeout)
	v.SetDefault("video.watchdog_interval", defaultVideoWatchdogInterval)
	v.SetDefault("video.hls_url_ttl", defaultVideoHLSURLTTL)
	v.SetDefault("video.hls_segment_ttl", defaultVideoHLSSegmentTTL)
}
