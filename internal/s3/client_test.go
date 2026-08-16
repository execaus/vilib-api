package s3_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
	"vilib-api/config"
	"vilib-api/internal/s3"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	miniomodule "github.com/testcontainers/testcontainers-go/modules/minio"
)

// Учётные данные и параметры тестового MinIO. Один контейнер на весь пакет — поднимается
// в TestMain, адрес передаётся тестам через переменную окружения (без пакетных var).
const (
	envTestEndpoint = "S3_CLIENT_TEST_ENDPOINT"
	testAccessKey   = "minioadmin"
	testSecretKey   = "minioadmin"
	testRegion      = "us-east-1"
	testPresignTTL  = 5 * time.Minute

	deleteByPrefixObjectCount = 1005
	deleteByPrefixWorkers     = 50

	mismatchBodySuffix = " and a bit more to change the length"
)

// TestMain поднимает единственный на пакет контейнер MinIO и передаёт адрес тестам через env.
func TestMain(m *testing.M) {
	os.Exit(runWithMinio(m))
}

func runWithMinio(m *testing.M) int {
	ctx := context.Background()

	container, err := miniomodule.Run(ctx, "minio/minio:RELEASE.2025-09-07T16-13-09Z",
		miniomodule.WithUsername(testAccessKey),
		miniomodule.WithPassword(testSecretKey),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to start minio container:", err)
		return 1
	}
	defer func() {
		if terminateErr := testcontainers.TerminateContainer(container); terminateErr != nil {
			fmt.Fprintln(os.Stderr, "failed to terminate minio container:", terminateErr)
		}
	}()

	endpoint, err := container.ConnectionString(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to get minio connection string:", err)
		return 1
	}

	if setenvErr := os.Setenv(envTestEndpoint, "http://"+endpoint); setenvErr != nil {
		fmt.Fprintln(os.Stderr, "failed to set endpoint env:", setenvErr)
		return 1
	}

	return m.Run()
}

// testEndpoint возвращает адрес тестового MinIO, поднятого в TestMain.
func testEndpoint(t *testing.T) string {
	t.Helper()

	endpoint := os.Getenv(envTestEndpoint)
	require.NotEmpty(t, endpoint, "переменная окружения %s не установлена: TestMain не запустился", envTestEndpoint)

	return endpoint
}

// newTestClient создаёт клиент под тестируемым internal/s3.Client с заданным публичным адресом.
// Если publicEndpoint пуст, используется тот же адрес, что и для внутренних операций.
func newTestClient(t *testing.T, publicEndpoint string) *s3.Client {
	t.Helper()

	endpoint := testEndpoint(t)
	if publicEndpoint == "" {
		publicEndpoint = endpoint
	}

	client, err := s3.NewClient(config.S3Config{
		Endpoint:        endpoint,
		PublicEndpoint:  publicEndpoint,
		AccessKeyID:     testAccessKey,
		SecretAccessKey: testSecretKey,
		Region:          testRegion,
		UsePathStyle:    true,
	})
	require.NoError(t, err)

	return client
}

// newRawAWSClient создаёт «сырой» клиент aws-sdk-go-v2 для подготовки тестовых данных
// (создание бакета, загрузка объектов) в обход тестируемого internal/s3.Client.
func newRawAWSClient(t *testing.T) *awss3.Client {
	t.Helper()

	awsCfg, err := awsconfig.LoadDefaultConfig(
		t.Context(),
		awsconfig.WithRegion(testRegion),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(testAccessKey, testSecretKey, "")),
	)
	require.NoError(t, err)

	return awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		o.BaseEndpoint = aws.String(testEndpoint(t))
		o.UsePathStyle = true
	})
}

// newTestBucket создаёт уникальный бакет и возвращает его имя.
func newTestBucket(t *testing.T, raw *awss3.Client) string {
	t.Helper()

	bucket := "test-" + strings.ReplaceAll(uuid.NewString(), "-", "")

	_, err := raw.CreateBucket(t.Context(), &awss3.CreateBucketInput{Bucket: aws.String(bucket)})
	require.NoError(t, err)

	return bucket
}

// putRawObject загружает объект в обход тестируемого клиента — для подготовки данных теста.
func putRawObject(ctx context.Context, raw *awss3.Client, bucket, key, contentType string, body []byte) error {
	_, err := raw.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to put raw object: %w", err)
	}

	return nil
}

// httpPut выполняет HTTP PUT по presigned-ссылке с заданным телом и Content-Type.
func httpPut(t *testing.T, url, contentType string, body []byte) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", contentType)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, resp.Body.Close()) })

	return resp
}

func TestClient_PresignPutObject_AcceptsMatchingSizeAndRejectsMismatch(t *testing.T) {
	t.Parallel()

	raw := newRawAWSClient(t)
	bucket := newTestBucket(t, raw)
	client := newTestClient(t, "")

	const contentType = "video/mp4"
	body := []byte("presigned upload payload")
	key := "videos/" + uuid.NewString() + "/original"

	url, err := client.PresignPutObject(t.Context(), bucket, key, contentType, int64(len(body)), testPresignTTL)
	require.NoError(t, err)

	resp := httpPut(t, string(url), contentType, body)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	info, err := client.HeadObject(t.Context(), bucket, key)
	require.NoError(t, err)
	require.Equal(t, int64(len(body)), info.Size)
	require.Equal(t, contentType, info.ContentType)

	mismatchKey := "videos/" + uuid.NewString() + "/original"
	mismatchURL, err := client.PresignPutObject(
		t.Context(), bucket, mismatchKey, contentType, int64(len(body)), testPresignTTL,
	)
	require.NoError(t, err)

	// Тело другого размера, чем подписано в ссылке, — заголовок Content-Length,
	// автоматически выставленный клиентом, разойдётся с подписью.
	otherBody := make([]byte, 0, len(body)+len(mismatchBodySuffix))
	otherBody = append(otherBody, body...)
	otherBody = append(otherBody, mismatchBodySuffix...)
	mismatchResp := httpPut(t, string(mismatchURL), contentType, otherBody)
	require.Equal(t, http.StatusForbidden, mismatchResp.StatusCode)
}

func TestClient_PresignGetObject_StreamsStoredBody(t *testing.T) {
	t.Parallel()

	raw := newRawAWSClient(t)
	bucket := newTestBucket(t, raw)
	client := newTestClient(t, "")

	body := []byte("streamed playlist content")
	key := "videos/" + uuid.NewString() + "/hls/master.m3u8"
	require.NoError(t, putRawObject(t.Context(), raw, bucket, key, "application/vnd.apple.mpegurl", body))

	url, err := client.PresignGetObject(t.Context(), bucket, key, testPresignTTL)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, string(url), nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, resp.Body.Close()) })

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Empty(t, resp.Header.Get("Content-Disposition"))

	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, body, got)
}

func TestClient_GetObject_ReturnsStoredBody(t *testing.T) {
	t.Parallel()

	raw := newRawAWSClient(t)
	bucket := newTestBucket(t, raw)
	client := newTestClient(t, "")

	body := []byte("small object content")
	key := "videos/" + uuid.NewString() + "/hls/master.m3u8"
	require.NoError(t, putRawObject(t.Context(), raw, bucket, key, "application/vnd.apple.mpegurl", body))

	got, err := client.GetObject(t.Context(), bucket, key)
	require.NoError(t, err)
	require.Equal(t, body, got)
}

func TestClient_HeadObject_MissingKeyReturnsErrObjectNotFound(t *testing.T) {
	t.Parallel()

	raw := newRawAWSClient(t)
	bucket := newTestBucket(t, raw)
	client := newTestClient(t, "")

	_, err := client.HeadObject(t.Context(), bucket, "videos/"+uuid.NewString()+"/original")
	require.ErrorIs(t, err, s3.ErrObjectNotFound)
}

func TestClient_GetObject_MissingKeyReturnsErrObjectNotFound(t *testing.T) {
	t.Parallel()

	raw := newRawAWSClient(t)
	bucket := newTestBucket(t, raw)
	client := newTestClient(t, "")

	_, err := client.GetObject(t.Context(), bucket, "videos/"+uuid.NewString()+"/hls/master.m3u8")
	require.ErrorIs(t, err, s3.ErrObjectNotFound)
}

func TestClient_DeleteByPrefix_RemovesOnlyMatchingObjects(t *testing.T) {
	t.Parallel()

	raw := newRawAWSClient(t)
	bucket := newTestBucket(t, raw)
	client := newTestClient(t, "")

	videoID := uuid.NewString()
	targetPrefix := "videos/" + videoID + "/"
	keepKey := "videos/" + uuid.NewString() + "/original"

	require.NoError(t, putRawObject(t.Context(), raw, bucket, keepKey, "application/octet-stream", []byte("keep")))
	uploadManyObjects(t, raw, bucket, targetPrefix, deleteByPrefixObjectCount)

	deleted, err := client.DeleteByPrefix(t.Context(), bucket, targetPrefix)
	require.NoError(t, err)
	require.Equal(t, deleteByPrefixObjectCount, deleted)

	listOut, err := raw.ListObjectsV2(t.Context(), &awss3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(targetPrefix),
	})
	require.NoError(t, err)
	require.Empty(t, listOut.Contents)

	_, err = client.HeadObject(t.Context(), bucket, keepKey)
	require.NoError(t, err)
}

// uploadManyObjects параллельно загружает count мелких объектов с общим префиксом ключа.
func uploadManyObjects(t *testing.T, raw *awss3.Client, bucket, prefix string, count int) {
	t.Helper()

	jobs := make(chan int)
	errCh := make(chan error, count)

	var wg sync.WaitGroup
	for range deleteByPrefixWorkers {
		wg.Go(func() {
			for i := range jobs {
				key := fmt.Sprintf("%sobj-%05d.txt", prefix, i)
				if err := putRawObject(
					t.Context(),
					raw,
					bucket,
					key,
					"application/octet-stream",
					[]byte("x"),
				); err != nil {
					errCh <- err
				}
			}
		})
	}

	for i := range count {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}
}

func TestClient_PresignedURL_HostMatchesPublicEndpointNotInternalEndpoint(t *testing.T) {
	t.Parallel()

	const fakePublicEndpoint = "http://public.example:9000"

	raw := newRawAWSClient(t)
	bucket := newTestBucket(t, raw)
	client := newTestClient(t, fakePublicEndpoint)

	key := "videos/" + uuid.NewString() + "/original"

	putURL, err := client.PresignPutObject(t.Context(), bucket, key, "video/mp4", 4, testPresignTTL)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(putURL), fakePublicEndpoint+"/"))

	getURL, err := client.PresignGetObject(t.Context(), bucket, key, testPresignTTL)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(getURL), fakePublicEndpoint+"/"))

	require.NotEqual(t, fakePublicEndpoint, testEndpoint(t))
}
