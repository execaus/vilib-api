package s3

import (
	"context"
	"errors"
	"time"
	"vilib-api/internal/domain"
)

// ErrObjectNotFound возвращается, если объект не найден в хранилище.
var ErrObjectNotFound = errors.New("object not found")

// ObjectInfo — метаданные объекта в хранилище.
type ObjectInfo struct {
	Size        int64
	ContentType string
	ETag        string
}

type S3 interface {
	// PresignPutObject — URL на загрузку; Content-Type и Content-Length входят в подпись.
	PresignPutObject(
		ctx context.Context,
		bucket, key, contentType string,
		size int64,
		ttl time.Duration,
	) (domain.PreflightURL, error)
	// PresignGetObject — URL на потоковое чтение.
	PresignGetObject(ctx context.Context, bucket, key string, ttl time.Duration) (domain.PreflightURL, error)
	// HeadObject возвращает метаданные объекта, ErrObjectNotFound при отсутствии.
	HeadObject(ctx context.Context, bucket, key string) (ObjectInfo, error)
	// GetObject читает содержимое объекта целиком (только небольшие объекты, например плейлисты).
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
	DeleteObject(ctx context.Context, bucket, key string) error
	// DeleteByPrefix удаляет все объекты с указанным префиксом ключа, возвращает их количество.
	DeleteByPrefix(ctx context.Context, bucket, prefix string) (int, error)
}
