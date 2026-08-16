package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
	"vilib-api/config"
	"vilib-api/internal/domain"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// deleteBatchSize — максимальное количество объектов в одном запросе DeleteObjects (лимит S3 API).
const deleteBatchSize = 1000

// Client — реализация [S3] поверх aws-sdk-go-v2.
//
// Для операций с объектами (Head/Get/Delete/List) используется внутренний endpoint хранилища,
// для генерации presigned-ссылок — публичный: хост входит в подпись SigV4, поэтому подписывать
// нужно тем адресом, по которому ссылку откроет клиент (см. Э1-Т5 эпика).
type Client struct {
	inner   *awss3.Client
	presign *awss3.PresignClient
}

// NewClient создаёт клиент S3-совместимого хранилища по конфигурации.
func NewClient(cfg config.S3Config) (*Client, error) {
	creds := credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")

	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(creds),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	inner := awss3.NewFromConfig(awsCfg, withEndpoint(cfg, cfg.Endpoint))
	presignSource := awss3.NewFromConfig(awsCfg, withEndpoint(cfg, cfg.PublicEndpoint))

	return &Client{
		inner:   inner,
		presign: awss3.NewPresignClient(presignSource),
	}, nil
}

// withEndpoint возвращает функциональную опцию клиента S3 с заданным базовым адресом.
func withEndpoint(cfg config.S3Config, endpoint string) func(*awss3.Options) {
	return func(o *awss3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = cfg.UsePathStyle
		// Новые версии SDK по умолчанию добавляют заголовки контрольной суммы CRC,
		// которые MinIO и другие S3-совместимые хранилища отвергают.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	}
}

// PresignPutObject возвращает URL на загрузку объекта; Content-Type и Content-Length
// входят в подпись — клиент обязан прислать точно такие же заголовки.
func (c *Client) PresignPutObject(
	ctx context.Context,
	bucket, key, contentType string,
	size int64,
	ttl time.Duration,
) (domain.PreflightURL, error) {
	req, err := c.presign.PresignPutObject(ctx, &awss3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	}, awss3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("failed to presign put object: %w", err)
	}

	return domain.PreflightURL(req.URL), nil
}

// PresignGetObject возвращает URL на потоковое чтение объекта, без Content-Disposition.
func (c *Client) PresignGetObject(
	ctx context.Context,
	bucket, key string,
	ttl time.Duration,
) (domain.PreflightURL, error) {
	req, err := c.presign.PresignGetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, awss3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("failed to presign get object: %w", err)
	}

	return domain.PreflightURL(req.URL), nil
}

// HeadObject возвращает метаданные объекта, [ErrObjectNotFound] при отсутствии.
func (c *Client) HeadObject(ctx context.Context, bucket, key string) (ObjectInfo, error) {
	out, err := c.inner.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFoundErr(err) {
			return ObjectInfo{}, fmt.Errorf("%w: %s/%s: %w", ErrObjectNotFound, bucket, key, err)
		}

		return ObjectInfo{}, fmt.Errorf("failed to head object: %w", err)
	}

	return ObjectInfo{
		Size:        aws.ToInt64(out.ContentLength),
		ContentType: aws.ToString(out.ContentType),
		ETag:        aws.ToString(out.ETag),
	}, nil
}

// GetObject читает содержимое объекта целиком. Предназначен только для небольших объектов
// (например, HLS-плейлистов) — без ограничения на размер, без потоковой отдачи.
func (c *Client) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	out, err := c.inner.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFoundErr(err) {
			return nil, fmt.Errorf("%w: %s/%s: %w", ErrObjectNotFound, bucket, key, err)
		}

		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object body: %w", err)
	}

	return data, nil
}

// DeleteObject удаляет один объект.
func (c *Client) DeleteObject(ctx context.Context, bucket, key string) error {
	if _, err := c.inner.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// DeleteByPrefix удаляет все объекты с указанным префиксом ключа батчами по [deleteBatchSize],
// возвращает их количество.
func (c *Client) DeleteByPrefix(ctx context.Context, bucket, prefix string) (int, error) {
	paginator := awss3.NewListObjectsV2Paginator(c.inner, &awss3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	deleted := 0

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return deleted, fmt.Errorf("failed to list objects by prefix %q: %w", prefix, err)
		}

		for start := 0; start < len(page.Contents); start += deleteBatchSize {
			end := min(start+deleteBatchSize, len(page.Contents))

			count, batchErr := c.deleteObjectsBatch(ctx, bucket, page.Contents[start:end])
			deleted += count
			if batchErr != nil {
				return deleted, batchErr
			}
		}
	}

	return deleted, nil
}

// deleteObjectsBatch удаляет один батч объектов (не более [deleteBatchSize]) и возвращает
// число фактически удалённых объектов.
func (c *Client) deleteObjectsBatch(ctx context.Context, bucket string, objects []types.Object) (int, error) {
	if len(objects) == 0 {
		return 0, nil
	}

	ids := make([]types.ObjectIdentifier, 0, len(objects))
	for _, obj := range objects {
		ids = append(ids, types.ObjectIdentifier{Key: obj.Key})
	}

	out, err := c.inner.DeleteObjects(ctx, &awss3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{Objects: ids, Quiet: aws.Bool(true)},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to delete objects batch: %w", err)
	}

	if len(out.Errors) > 0 {
		first := out.Errors[0]
		return 0, fmt.Errorf(
			"failed to delete object %q: %s", aws.ToString(first.Key), aws.ToString(first.Message),
		)
	}

	return len(ids), nil
}

// isNotFoundErr определяет, соответствует ли ошибка отсутствию объекта в хранилище.
func isNotFoundErr(err error) bool {
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}

	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey":
			return true
		}
	}

	return false
}
