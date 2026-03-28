package s3

import (
	"context"
	"time"
	"vilib-api/internal/domain"

	"github.com/google/uuid"
)

type S3 interface {
	GetPreflightURL(
		ctx context.Context,
		bucketName domain.VideoBucket,
		assetID uuid.UUID,
		ttl time.Duration,
	) (domain.PreflightURL, error)
	GetPreflightUploadURL(
		ctx context.Context,
		bucketName domain.VideoBucket,
		fileID uuid.UUID,
		ttl time.Duration,
	) (domain.PreflightURL, error)
}
