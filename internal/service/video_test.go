package service_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/dberrors"
	"vilib-api/internal/repository/repository_mocks"
	"vilib-api/internal/s3"
	"vilib-api/internal/saga"
	"vilib-api/internal/saga/saga_mocks"
	"vilib-api/internal/service"
	"vilib-api/internal/service/service_mocks"

	"github.com/gojuno/minimock/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	events "github.com/execaus/vilib-events"
)

// videoConfig собирает минимальный config.VideoConfig для тестов сервиса видео.
func videoConfig(maxUploadSizeBytes int64) config.VideoConfig {
	return config.VideoConfig{MaxUploadSizeBytes: maxUploadSizeBytes}
}

// videoCreateUploadMocks собирает моки, используемые CreateUpload.
type videoCreateUploadMocks struct {
	Access      *service_mocks.AccessMock
	GroupMember *service_mocks.GroupMemberMock
	GroupRole   *service_mocks.GroupRoleMock
	S3          *service_mocks.S3Mock
	Video       *repository_mocks.VideoMock
}

func TestService_Video_CreateUpload(t *testing.T) {
	t.Parallel()

	var (
		testAccountID    = uuid.New()
		testGroupID      = uuid.New()
		testUserID       = uuid.New()
		testVideoID      = uuid.New()
		testName         = "test video"
		testContentType  = "video/mp4"
		testSize         = int64(1024)
		testMaxSize      = int64(4 << 30)
		testKey          = "videos/" + testVideoID.String() + "/original"
		testBucket       = "vilib"
		testPreflightURL = domain.PreflightURL("https://example.com/upload")
	)

	testVideo := domain.Video{
		ID:      testVideoID,
		GroupID: testGroupID,
		Name:    testName,
		Author:  testUserID,
		Status:  domain.VideoStatusUploading,
	}

	tests := []struct {
		name              string
		contentType       string
		size              int64
		setupMocks        func(m videoCreateUploadMocks)
		wantVideoID       uuid.UUID
		wantUploadURL     domain.PreflightURL
		wantErr           error
		wantValidationErr bool
	}{
		{
			name:        "success - account permission",
			contentType: testContentType,
			size:        testSize,
			setupMocks: func(m videoCreateUploadMocks) {
				m.Access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionManageVideo).
					Return(nil)
				m.Video.InsertMock.
					Expect(minimock.AnyContext, testName, testGroupID, testUserID, domain.VideoStatusUploading).
					Return(testVideo, nil)
				m.S3.PresignPutObjectMock.
					Expect(
						minimock.AnyContext,
						testBucket,
						testKey,
						testContentType,
						testSize,
						domain.VideoUploadURLTTL,
					).
					Return(testPreflightURL, nil)
			},
			wantVideoID:   testVideoID,
			wantUploadURL: testPreflightURL,
		},
		{
			name:        "success - group permission only",
			contentType: testContentType,
			size:        testSize,
			setupMocks: func(m videoCreateUploadMocks) {
				groupRoleID := uuid.New()
				m.Access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionManageVideo).
					Return(service.ErrForbidden)
				m.GroupMember.GetByUserIDAndGroupIDMock.
					Expect(minimock.AnyContext, testUserID, testGroupID).
					Return(domain.GroupMember{UserID: testUserID, RoleID: groupRoleID}, nil)
				m.GroupRole.GetByIDMock.
					Expect(minimock.AnyContext, groupRoleID).
					Return([]domain.GroupRole{{PermissionMask: domain.PermissionMask(1 << domain.GroupPermissionManageVideo)}}, nil)
				m.Video.InsertMock.
					Expect(minimock.AnyContext, testName, testGroupID, testUserID, domain.VideoStatusUploading).
					Return(testVideo, nil)
				m.S3.PresignPutObjectMock.
					Expect(
						minimock.AnyContext,
						testBucket,
						testKey,
						testContentType,
						testSize,
						domain.VideoUploadURLTTL,
					).
					Return(testPreflightURL, nil)
			},
			wantVideoID:   testVideoID,
			wantUploadURL: testPreflightURL,
		},
		{
			name:        "access denied",
			contentType: testContentType,
			size:        testSize,
			setupMocks: func(m videoCreateUploadMocks) {
				m.Access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionManageVideo).
					Return(service.ErrForbidden)
				m.GroupMember.GetByUserIDAndGroupIDMock.
					Expect(minimock.AnyContext, testUserID, testGroupID).
					Return(domain.GroupMember{}, service.ErrForbidden)
			},
			wantErr: service.ErrForbidden,
		},
		{
			name:        "invalid content type",
			contentType: "application/octet-stream",
			size:        testSize,
			setupMocks: func(m videoCreateUploadMocks) {
				m.Access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionManageVideo).
					Return(nil)
			},
			wantValidationErr: true,
		},
		{
			name:        "size exceeds limit",
			contentType: testContentType,
			size:        testMaxSize + 1,
			setupMocks: func(m videoCreateUploadMocks) {
				m.Access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionManageVideo).
					Return(nil)
			},
			wantValidationErr: true,
		},
		{
			name:        "size is zero",
			contentType: testContentType,
			size:        0,
			setupMocks: func(m videoCreateUploadMocks) {
				m.Access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionManageVideo).
					Return(nil)
			},
			wantValidationErr: true,
		},
		{
			name:        "duplicate video name",
			contentType: testContentType,
			size:        testSize,
			setupMocks: func(m videoCreateUploadMocks) {
				m.Access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionManageVideo).
					Return(nil)
				m.Video.InsertMock.
					Expect(minimock.AnyContext, testName, testGroupID, testUserID, domain.VideoStatusUploading).
					Return(domain.Video{}, dberrors.UserGroupVideoErrors.ErrUniqueUserGroupVideosUserGroupIdNameKey)
			},
			wantErr: service.NewConflictError("video name already exists"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mc := minimock.NewController(t)

			m := videoCreateUploadMocks{
				Access:      service_mocks.NewAccessMock(mc),
				GroupMember: service_mocks.NewGroupMemberMock(mc),
				GroupRole:   service_mocks.NewGroupRoleMock(mc),
				S3:          service_mocks.NewS3Mock(mc),
				Video:       repository_mocks.NewVideoMock(mc),
			}

			tt.setupMocks(m)

			svc := service.Service{
				Access:      m.Access,
				GroupMember: m.GroupMember,
				GroupRole:   m.GroupRole,
			}

			videoSvc := service.NewVideoService(m.S3, m.Video, &svc, service.VideoServiceConfig{
				Bucket: testBucket,
				Video:  videoConfig(testMaxSize),
			})

			got, err := videoSvc.CreateUpload(
				minimock.AnyContext,
				testAccountID, testGroupID, testUserID,
				testName, tt.contentType, tt.size,
			)

			if tt.wantValidationErr {
				var validationErr *service.ValidationError
				require.ErrorAs(t, err, &validationErr)
				return
			}

			require.ErrorIs(t, err, tt.wantErr)
			if tt.wantErr != nil {
				return
			}

			require.Equal(t, tt.wantVideoID, got.VideoID)
			require.Equal(t, tt.wantUploadURL, got.UploadURL)
			require.WithinDuration(t, time.Now().Add(domain.VideoUploadURLTTL), got.ExpiresAt, time.Second)
		})
	}
}

// videoCompleteUploadMocks собирает моки, используемые CompleteUpload.
type videoCompleteUploadMocks struct {
	Access      *service_mocks.AccessMock
	GroupMember *service_mocks.GroupMemberMock
	GroupRole   *service_mocks.GroupRoleMock
	S3          *service_mocks.S3Mock
	Video       *repository_mocks.VideoMock
	VideoAsset  *service_mocks.VideoAssetMock
	Outbox      *service_mocks.OutboxMock
}

func TestService_Video_CompleteUpload(t *testing.T) {
	t.Parallel()

	var (
		testAccountID = uuid.New()
		testGroupID   = uuid.New()
		testUserID    = uuid.New()
		testVideoID   = uuid.New()
		testBucket    = "vilib"
		testKey       = "videos/" + testVideoID.String() + "/original"
		testTopic     = "video.original-uploaded"
	)

	uploadingVideo := domain.Video{ID: testVideoID, GroupID: testGroupID, Status: domain.VideoStatusUploading}
	queuedVideo := domain.Video{
		ID:                testVideoID,
		GroupID:           testGroupID,
		Status:            domain.VideoStatusQueued,
		ProcessingAttempt: 1,
	}
	readyVideo := domain.Video{ID: testVideoID, GroupID: testGroupID, Status: domain.VideoStatusReady}
	failureReason := "timeout"
	failedVideo := domain.Video{
		ID: testVideoID, GroupID: testGroupID, Status: domain.VideoStatusFailed, FailureReason: &failureReason,
	}
	otherGroupVideo := domain.Video{ID: testVideoID, GroupID: uuid.New(), Status: domain.VideoStatusUploading}

	tests := []struct {
		name       string
		setupMocks func(t *testing.T, m videoCompleteUploadMocks)
		want       domain.Video
		wantErr    error
	}{
		{
			name: "success",
			setupMocks: func(t *testing.T, m videoCompleteUploadMocks) {
				m.Access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionManageVideo).
					Return(nil)

				selectCalls := 0
				m.Video.SelectMock.Set(func(_ context.Context, _ uuid.UUID) (*domain.Video, error) {
					selectCalls++
					if selectCalls == 1 {
						return &uploadingVideo, nil
					}
					return &queuedVideo, nil
				})

				m.S3.HeadObjectMock.
					Expect(minimock.AnyContext, testBucket, testKey).
					Return(s3.ObjectInfo{Size: 2048, ContentType: "video/mp4"}, nil)
				m.VideoAsset.CreateMock.
					Expect(
						minimock.AnyContext, testVideoID, domain.VideoAssetKindOriginal, domain.VideoProfile(""),
						testBucket, testKey, "video/mp4", int64(2048),
					).
					Return(domain.VideoAsset{}, nil)

				attempt := 1
				m.Video.UpdateStatusIfMock.
					Expect(
						minimock.AnyContext, testVideoID,
						[]domain.VideoStatus{domain.VideoStatusUploading}, domain.VideoStatusQueued,
						domain.VideoPatch{ProcessingAttempt: &attempt},
					).
					Return(true, nil)

				m.Outbox.PublishMock.Set(func(_ context.Context, topic, key string, payload []byte) error {
					require.Equal(t, testTopic, topic)
					require.Equal(t, testVideoID.String(), key)

					envelope, err := events.Unmarshal(payload)
					require.NoError(t, err)
					require.Equal(t, events.TypeOriginalUploaded, envelope.EventType)
					require.Equal(t, 1, envelope.Attempt)
					require.Equal(t, testVideoID, envelope.VideoID)

					uploaded, err := envelope.OriginalUploaded()
					require.NoError(t, err)
					require.Equal(t, testBucket, uploaded.Bucket)
					require.Equal(t, testKey, uploaded.Key)
					require.Equal(t, "video/mp4", uploaded.ContentType)
					require.Equal(t, int64(2048), uploaded.SizeBytes)

					return nil
				})
			},
			want: queuedVideo,
		},
		{
			name: "object not found",
			setupMocks: func(_ *testing.T, m videoCompleteUploadMocks) {
				m.Access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionManageVideo).
					Return(nil)
				m.Video.SelectMock.Expect(minimock.AnyContext, testVideoID).Return(&uploadingVideo, nil)
				m.S3.HeadObjectMock.
					Expect(minimock.AnyContext, testBucket, testKey).
					Return(s3.ObjectInfo{}, s3.ErrObjectNotFound)
			},
			wantErr: service.NewConflictError("object not found in storage"),
		},
		{
			name: "object is empty",
			setupMocks: func(_ *testing.T, m videoCompleteUploadMocks) {
				m.Access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionManageVideo).
					Return(nil)
				m.Video.SelectMock.Expect(minimock.AnyContext, testVideoID).Return(&uploadingVideo, nil)
				m.S3.HeadObjectMock.
					Expect(minimock.AnyContext, testBucket, testKey).
					Return(s3.ObjectInfo{Size: 0, ContentType: "video/mp4"}, nil)
			},
			wantErr: service.NewConflictError("object is empty"),
		},
		{
			name: "repeat for queued video is idempotent",
			setupMocks: func(_ *testing.T, m videoCompleteUploadMocks) {
				m.Access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionManageVideo).
					Return(nil)
				m.Video.SelectMock.Expect(minimock.AnyContext, testVideoID).Return(&queuedVideo, nil)
			},
			want: queuedVideo,
		},
		{
			name: "repeat for ready video is idempotent",
			setupMocks: func(_ *testing.T, m videoCompleteUploadMocks) {
				m.Access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionManageVideo).
					Return(nil)
				m.Video.SelectMock.Expect(minimock.AnyContext, testVideoID).Return(&readyVideo, nil)
			},
			want: readyVideo,
		},
		{
			name: "failed video returns conflict",
			setupMocks: func(_ *testing.T, m videoCompleteUploadMocks) {
				m.Access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionManageVideo).
					Return(nil)
				m.Video.SelectMock.Expect(minimock.AnyContext, testVideoID).Return(&failedVideo, nil)
			},
			wantErr: service.NewConflictError("upload failed: timeout"),
		},
		{
			name: "video belongs to another group",
			setupMocks: func(_ *testing.T, m videoCompleteUploadMocks) {
				m.Access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionManageVideo).
					Return(nil)
				m.Video.SelectMock.Expect(minimock.AnyContext, testVideoID).Return(&otherGroupVideo, nil)
			},
			wantErr: service.ErrForbidden,
		},
		{
			name: "access denied",
			setupMocks: func(_ *testing.T, m videoCompleteUploadMocks) {
				m.Access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionManageVideo).
					Return(service.ErrForbidden)
				m.GroupMember.GetByUserIDAndGroupIDMock.
					Expect(minimock.AnyContext, testUserID, testGroupID).
					Return(domain.GroupMember{}, service.ErrForbidden)
			},
			wantErr: service.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mc := minimock.NewController(t)

			m := videoCompleteUploadMocks{
				Access:      service_mocks.NewAccessMock(mc),
				GroupMember: service_mocks.NewGroupMemberMock(mc),
				GroupRole:   service_mocks.NewGroupRoleMock(mc),
				S3:          service_mocks.NewS3Mock(mc),
				Video:       repository_mocks.NewVideoMock(mc),
				VideoAsset:  service_mocks.NewVideoAssetMock(mc),
				Outbox:      service_mocks.NewOutboxMock(mc),
			}

			tt.setupMocks(t, m)

			svc := service.Service{
				Access:      m.Access,
				GroupMember: m.GroupMember,
				GroupRole:   m.GroupRole,
				VideoAsset:  m.VideoAsset,
				Outbox:      m.Outbox,
			}

			videoSvc := service.NewVideoService(m.S3, m.Video, &svc, service.VideoServiceConfig{
				Bucket:                testBucket,
				TopicOriginalUploaded: testTopic,
				Video:                 videoConfig(4 << 30),
			})

			got, err := videoSvc.CompleteUpload(
				minimock.AnyContext,
				testAccountID,
				testGroupID,
				testUserID,
				testVideoID,
			)

			require.ErrorIs(t, err, tt.wantErr)
			if tt.wantErr != nil {
				return
			}

			require.Equal(t, tt.want, got)
		})
	}
}

// videoApplyProcessingMocks собирает моки, используемые Apply* методами обработки событий
// воркера (§7.2 эпика).
type videoApplyProcessingMocks struct {
	Video      *repository_mocks.VideoMock
	VideoAsset *service_mocks.VideoAssetMock
	Outbox     *service_mocks.OutboxMock
	S3         *service_mocks.S3Mock
}

func newVideoApplyProcessingMocks(mc *minimock.Controller) videoApplyProcessingMocks {
	return videoApplyProcessingMocks{
		Video:      repository_mocks.NewVideoMock(mc),
		VideoAsset: service_mocks.NewVideoAssetMock(mc),
		Outbox:     service_mocks.NewOutboxMock(mc),
		S3:         service_mocks.NewS3Mock(mc),
	}
}

// newVideoApplyService собирает VideoService поверх мока репозитория видео и моков
// межсервисных зависимостей (VideoAsset, Outbox), используемых Apply*-методами.
func newVideoApplyService(m videoApplyProcessingMocks, cfg service.VideoServiceConfig) *service.VideoService {
	svc := &service.Service{VideoAsset: m.VideoAsset, Outbox: m.Outbox}
	return service.NewVideoService(m.S3, m.Video, svc, cfg)
}

// testMaxProcessingAttempts — лимит попыток обработки, общий для тестов ApplyProcessingFailed.
const testMaxProcessingAttempts = 3

// videoProcessingConfig собирает config.VideoConfig с лимитом попыток обработки —
// единственным полем, которое использует ApplyProcessingFailed.
func videoProcessingConfig() config.VideoConfig {
	return config.VideoConfig{MaxProcessingAttempts: testMaxProcessingAttempts}
}

func TestService_Video_ApplyProcessingStarted(t *testing.T) {
	t.Parallel()

	testVideoID := uuid.New()
	repoErr := errors.New("db unavailable")

	tests := []struct {
		name       string
		attempt    int
		setupMocks func(m videoApplyProcessingMocks, attempt int)
		wantErr    error
	}{
		{
			name:    "queued and attempt matches transitions to compressing",
			attempt: 2,
			setupMocks: func(m videoApplyProcessingMocks, attempt int) {
				m.Video.UpdateStatusIfMock.Set(func(
					_ context.Context,
					id uuid.UUID,
					from []domain.VideoStatus,
					to domain.VideoStatus,
					patch domain.VideoPatch,
				) (bool, error) {
					require.Equal(t, testVideoID, id)
					require.Equal(t, []domain.VideoStatus{domain.VideoStatusQueued}, from)
					require.Equal(t, domain.VideoStatusCompressing, to)
					require.NotNil(t, patch.ExpectedAttempt)
					require.Equal(t, attempt, *patch.ExpectedAttempt)
					return true, nil
				})
			},
		},
		{
			name:    "stale attempt is ignored without error",
			attempt: 1,
			setupMocks: func(m videoApplyProcessingMocks, _ int) {
				m.Video.UpdateStatusIfMock.Return(false, nil)
			},
		},
		{
			name:    "repository error propagates",
			attempt: 1,
			setupMocks: func(m videoApplyProcessingMocks, _ int) {
				m.Video.UpdateStatusIfMock.Return(false, repoErr)
			},
			wantErr: repoErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mc := minimock.NewController(t)
			m := newVideoApplyProcessingMocks(mc)
			tt.setupMocks(m, tt.attempt)

			videoSvc := newVideoApplyService(m, service.VideoServiceConfig{Video: videoConfig(4 << 30)})

			envelope := events.Envelope{VideoID: testVideoID, Attempt: tt.attempt}
			err := videoSvc.ApplyProcessingStarted(
				minimock.AnyContext, envelope, events.ProcessingStarted{WorkerID: "worker-1"},
			)

			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestService_Video_ApplyProcessingCompleted(t *testing.T) {
	t.Parallel()

	testVideoID := uuid.New()
	testBucket := "vilib"
	repoErr := errors.New("db unavailable")

	t.Run("success registers results and transitions to ready", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoApplyProcessingMocks(mc)

		attempt := 2
		durationMs := int64(125000)
		width, height := 1280, 720

		m.Video.UpdateStatusIfMock.Set(func(
			_ context.Context, id uuid.UUID, from []domain.VideoStatus, to domain.VideoStatus, patch domain.VideoPatch,
		) (bool, error) {
			require.Equal(t, testVideoID, id)
			require.ElementsMatch(
				t,
				[]domain.VideoStatus{domain.VideoStatusQueued, domain.VideoStatusCompressing},
				from,
			)
			require.Equal(t, domain.VideoStatusReady, to)
			require.NotNil(t, patch.ExpectedAttempt)
			require.Equal(t, attempt, *patch.ExpectedAttempt)
			require.True(t, patch.ClearFailure)
			require.NotNil(t, patch.DurationMs)
			require.Equal(t, durationMs, *patch.DurationMs)
			require.NotNil(t, patch.Width)
			require.Equal(t, width, *patch.Width)
			require.NotNil(t, patch.Height)
			require.Equal(t, height, *patch.Height)
			return true, nil
		})

		var calls []string

		m.VideoAsset.DeleteByVideoAndKindsMock.Set(func(
			_ context.Context, id uuid.UUID, kinds []domain.VideoAssetKind,
		) error {
			require.Equal(t, testVideoID, id)
			require.ElementsMatch(
				t, []domain.VideoAssetKind{domain.VideoAssetKindHLSMaster, domain.VideoAssetKindHLSVariant}, kinds,
			)
			calls = append(calls, "delete")
			return nil
		})

		m.VideoAsset.CreateMock.Set(func(
			_ context.Context,
			id uuid.UUID,
			kind domain.VideoAssetKind,
			profile domain.VideoProfile,
			_, _, _ string,
			_ int64,
		) (domain.VideoAsset, error) {
			require.Equal(t, testVideoID, id)
			calls = append(calls, string(kind)+":"+string(profile))
			return domain.VideoAsset{}, nil
		})

		videoSvc := newVideoApplyService(m, service.VideoServiceConfig{Bucket: testBucket, Video: videoConfig(4 << 30)})

		envelope := events.Envelope{VideoID: testVideoID, Attempt: attempt}
		payload := events.ProcessingCompleted{
			WorkerID: "worker-1",
			Results: []events.AssetResult{
				{
					Kind:        events.AssetKindHLSMaster,
					Bucket:      testBucket,
					Key:         "videos/" + testVideoID.String() + "/hls/master.m3u8",
					ContentType: "application/vnd.apple.mpegurl",
					SizeBytes:   512,
				},
				{
					Kind:        events.AssetKindHLSVariant,
					Profile:     "720p",
					Bucket:      testBucket,
					Key:         "videos/" + testVideoID.String() + "/hls/720p/playlist.m3u8",
					ContentType: "application/vnd.apple.mpegurl",
					SizeBytes:   2048,
				},
			},
			Metadata: events.VideoMetadata{DurationMs: durationMs, Width: width, Height: height},
		}

		err := videoSvc.ApplyProcessingCompleted(minimock.AnyContext, envelope, payload)

		require.NoError(t, err)
		require.Equal(t, []string{"delete", "hls_master:", "hls_variant:720p"}, calls)
	})

	t.Run("invalid asset kind returns validation error and stops registration", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoApplyProcessingMocks(mc)

		m.Video.UpdateStatusIfMock.Return(true, nil)
		m.VideoAsset.DeleteByVideoAndKindsMock.Return(nil)
		// VideoAsset.Create не настроен: невалидный kind обязан прерваться до его вызова —
		// неожиданный вызов упадёт через контроллер моков.

		videoSvc := newVideoApplyService(m, service.VideoServiceConfig{Bucket: testBucket, Video: videoConfig(4 << 30)})

		envelope := events.Envelope{VideoID: testVideoID, Attempt: 1}
		payload := events.ProcessingCompleted{
			Results: []events.AssetResult{{Kind: events.AssetKindOriginal, Bucket: testBucket, Key: "k"}},
		}

		err := videoSvc.ApplyProcessingCompleted(minimock.AnyContext, envelope, payload)

		var validationErr *service.ValidationError
		require.ErrorAs(t, err, &validationErr)
	})

	t.Run("repository error on status update propagates", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoApplyProcessingMocks(mc)
		m.Video.UpdateStatusIfMock.Return(false, repoErr)

		videoSvc := newVideoApplyService(m, service.VideoServiceConfig{Bucket: testBucket, Video: videoConfig(4 << 30)})

		err := videoSvc.ApplyProcessingCompleted(
			minimock.AnyContext, events.Envelope{VideoID: testVideoID, Attempt: 1}, events.ProcessingCompleted{},
		)

		require.ErrorIs(t, err, repoErr)
	})

	t.Run("delete by kinds error propagates", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoApplyProcessingMocks(mc)
		m.Video.UpdateStatusIfMock.Return(true, nil)
		m.VideoAsset.DeleteByVideoAndKindsMock.Return(repoErr)

		videoSvc := newVideoApplyService(m, service.VideoServiceConfig{Bucket: testBucket, Video: videoConfig(4 << 30)})

		err := videoSvc.ApplyProcessingCompleted(
			minimock.AnyContext, events.Envelope{VideoID: testVideoID, Attempt: 1}, events.ProcessingCompleted{},
		)

		require.ErrorIs(t, err, repoErr)
	})

	t.Run("ignored transition schedules orphan cleanup after commit", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoApplyProcessingMocks(mc)
		m.Video.UpdateStatusIfMock.Return(false, nil)

		var cleanedBucket, cleanedPrefix string
		m.S3.DeleteByPrefixMock.Set(func(_ context.Context, bucket, prefix string) (int, error) {
			cleanedBucket = bucket
			cleanedPrefix = prefix
			return 0, nil
		})

		videoSvc := newVideoApplyService(m, service.VideoServiceConfig{Bucket: testBucket, Video: videoConfig(4 << 30)})

		tx := saga_mocks.NewBobTransactionMock(mc)
		tx.CommitMock.Return(nil)
		tx.RollbackMock.Return(nil)
		tx.RollbackMock.Optional()

		repo := saga_mocks.NewTransactableMock(mc)
		repo.WithTxMock.Return(tx, nil)

		runner := saga.NewSagaRunner(videoSvc, repo)

		envelope := events.Envelope{VideoID: testVideoID, Attempt: 1}
		err := runner.Run(t.Context(), func(ctx context.Context, svc *service.VideoService) error {
			return svc.ApplyProcessingCompleted(ctx, envelope, events.ProcessingCompleted{})
		})

		require.NoError(t, err)
		require.Equal(t, testBucket, cleanedBucket)
		require.Equal(t, "videos/"+testVideoID.String()+"/hls/", cleanedPrefix)
	})
}

func TestService_Video_ApplyProcessingFailed(t *testing.T) {
	t.Parallel()

	testVideoID := uuid.New()
	testTopic := "video.processing-events-original-uploaded"
	repoErr := errors.New("db unavailable")

	t.Run("permanent error transitions to failed", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoApplyProcessingMocks(mc)

		attempt := 1
		m.Video.UpdateStatusIfMock.Set(func(
			_ context.Context, id uuid.UUID, from []domain.VideoStatus, to domain.VideoStatus, patch domain.VideoPatch,
		) (bool, error) {
			require.Equal(t, testVideoID, id)
			require.ElementsMatch(
				t,
				[]domain.VideoStatus{domain.VideoStatusQueued, domain.VideoStatusCompressing},
				from,
			)
			require.Equal(t, domain.VideoStatusFailed, to)
			require.Equal(t, attempt, *patch.ExpectedAttempt)
			require.Equal(t, domain.VideoFailureClassPermanent, *patch.FailureClass)
			require.Equal(t, "unsupported codec", *patch.FailureReason)
			return true, nil
		})

		videoSvc := newVideoApplyService(m, service.VideoServiceConfig{Video: videoProcessingConfig()})

		err := videoSvc.ApplyProcessingFailed(
			minimock.AnyContext,
			events.Envelope{VideoID: testVideoID, Attempt: attempt},
			events.ProcessingFailed{ErrorClass: events.ErrorClassPermanent, Reason: "unsupported codec"},
		)

		require.NoError(t, err)
	})

	t.Run("permanent error ignored on stale attempt", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoApplyProcessingMocks(mc)
		m.Video.UpdateStatusIfMock.Return(false, nil)

		videoSvc := newVideoApplyService(m, service.VideoServiceConfig{Video: videoProcessingConfig()})

		err := videoSvc.ApplyProcessingFailed(
			minimock.AnyContext,
			events.Envelope{VideoID: testVideoID, Attempt: 1},
			events.ProcessingFailed{ErrorClass: events.ErrorClassPermanent, Reason: "unsupported codec"},
		)

		require.NoError(t, err)
	})

	t.Run("temporary error with remaining attempts requeues and republishes original", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoApplyProcessingMocks(mc)

		attempt := 1
		next := 2
		originalKey := "videos/" + testVideoID.String() + "/original"

		m.Video.UpdateStatusIfMock.Set(func(
			_ context.Context, id uuid.UUID, from []domain.VideoStatus, to domain.VideoStatus, patch domain.VideoPatch,
		) (bool, error) {
			require.Equal(t, testVideoID, id)
			require.ElementsMatch(
				t,
				[]domain.VideoStatus{domain.VideoStatusQueued, domain.VideoStatusCompressing},
				from,
			)
			require.Equal(t, domain.VideoStatusQueued, to)
			require.Equal(t, attempt, *patch.ExpectedAttempt)
			require.Equal(t, next, *patch.ProcessingAttempt)
			return true, nil
		})

		m.VideoAsset.GetMock.Expect(minimock.AnyContext, testVideoID).Return([]domain.VideoAsset{
			{Kind: domain.VideoAssetKindOriginal, ObjectKey: originalKey, ContentType: "video/mp4", SizeBytes: 2048},
		}, nil)

		m.Outbox.PublishMock.Set(func(_ context.Context, topic, key string, payload []byte) error {
			require.Equal(t, testTopic, topic)
			require.Equal(t, testVideoID.String(), key)

			envelope, err := events.Unmarshal(payload)
			require.NoError(t, err)
			require.Equal(t, next, envelope.Attempt)

			uploaded, err := envelope.OriginalUploaded()
			require.NoError(t, err)
			require.Equal(t, originalKey, uploaded.Key)
			require.Equal(t, "video/mp4", uploaded.ContentType)
			require.Equal(t, int64(2048), uploaded.SizeBytes)

			return nil
		})

		videoSvc := newVideoApplyService(m, service.VideoServiceConfig{
			TopicOriginalUploaded: testTopic,
			Video:                 videoProcessingConfig(),
		})

		err := videoSvc.ApplyProcessingFailed(
			minimock.AnyContext,
			events.Envelope{VideoID: testVideoID, Attempt: attempt},
			events.ProcessingFailed{ErrorClass: events.ErrorClassTemporary, Reason: "network timeout"},
		)

		require.NoError(t, err)
	})

	t.Run("unknown error class is treated as temporary", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoApplyProcessingMocks(mc)

		attempt := 1
		next := 2
		originalKey := "videos/" + testVideoID.String() + "/original"

		m.Video.UpdateStatusIfMock.Set(func(
			_ context.Context, _ uuid.UUID, _ []domain.VideoStatus, to domain.VideoStatus, patch domain.VideoPatch,
		) (bool, error) {
			require.Equal(t, domain.VideoStatusQueued, to)
			require.NotNil(t, patch.ProcessingAttempt)
			require.Equal(t, next, *patch.ProcessingAttempt)
			return true, nil
		})
		m.VideoAsset.GetMock.Expect(minimock.AnyContext, testVideoID).Return([]domain.VideoAsset{
			{Kind: domain.VideoAssetKindOriginal, ObjectKey: originalKey, ContentType: "video/mp4", SizeBytes: 2048},
		}, nil)
		m.Outbox.PublishMock.Return(nil)

		videoSvc := newVideoApplyService(m, service.VideoServiceConfig{
			TopicOriginalUploaded: testTopic,
			Video:                 videoProcessingConfig(),
		})

		err := videoSvc.ApplyProcessingFailed(
			minimock.AnyContext,
			events.Envelope{VideoID: testVideoID, Attempt: attempt},
			events.ProcessingFailed{ErrorClass: "unknown-worker-error", Reason: "mystery"},
		)

		require.NoError(t, err)
	})

	t.Run("temporary error requeue ignored on stale attempt", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoApplyProcessingMocks(mc)
		m.Video.UpdateStatusIfMock.Return(false, nil)

		videoSvc := newVideoApplyService(m, service.VideoServiceConfig{Video: videoProcessingConfig()})

		err := videoSvc.ApplyProcessingFailed(
			minimock.AnyContext,
			events.Envelope{VideoID: testVideoID, Attempt: 1},
			events.ProcessingFailed{ErrorClass: events.ErrorClassTemporary, Reason: "network timeout"},
		)

		require.NoError(t, err)
	})

	t.Run("temporary error with attempts exhausted transitions to failed", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoApplyProcessingMocks(mc)

		attempt := testMaxProcessingAttempts
		m.Video.UpdateStatusIfMock.Set(func(
			_ context.Context, _ uuid.UUID, _ []domain.VideoStatus, to domain.VideoStatus, patch domain.VideoPatch,
		) (bool, error) {
			require.Equal(t, domain.VideoStatusFailed, to)
			require.Equal(t, attempt, *patch.ExpectedAttempt)
			require.Equal(t, domain.VideoFailureClassTemporary, *patch.FailureClass)
			require.Equal(t, "attempts exhausted: network timeout", *patch.FailureReason)
			return true, nil
		})

		videoSvc := newVideoApplyService(m, service.VideoServiceConfig{Video: videoProcessingConfig()})

		err := videoSvc.ApplyProcessingFailed(
			minimock.AnyContext,
			events.Envelope{VideoID: testVideoID, Attempt: attempt},
			events.ProcessingFailed{ErrorClass: events.ErrorClassTemporary, Reason: "network timeout"},
		)

		require.NoError(t, err)
	})

	t.Run("original asset missing when requeuing returns error", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoApplyProcessingMocks(mc)
		m.Video.UpdateStatusIfMock.Return(true, nil)
		m.VideoAsset.GetMock.Expect(minimock.AnyContext, testVideoID).Return(nil, nil)

		videoSvc := newVideoApplyService(m, service.VideoServiceConfig{Video: videoProcessingConfig()})

		err := videoSvc.ApplyProcessingFailed(
			minimock.AnyContext,
			events.Envelope{VideoID: testVideoID, Attempt: 1},
			events.ProcessingFailed{ErrorClass: events.ErrorClassTemporary, Reason: "network timeout"},
		)

		require.Error(t, err)
	})

	t.Run("repository error propagates", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoApplyProcessingMocks(mc)
		m.Video.UpdateStatusIfMock.Return(false, repoErr)

		videoSvc := newVideoApplyService(m, service.VideoServiceConfig{Video: videoProcessingConfig()})

		err := videoSvc.ApplyProcessingFailed(
			minimock.AnyContext,
			events.Envelope{VideoID: testVideoID, Attempt: 1},
			events.ProcessingFailed{ErrorClass: events.ErrorClassPermanent, Reason: "bad file"},
		)

		require.ErrorIs(t, err, repoErr)
	})
}

// videoGetMocks собирает моки, используемые Get.
type videoGetMocks struct {
	Access      *service_mocks.AccessMock
	GroupMember *service_mocks.GroupMemberMock
	GroupRole   *service_mocks.GroupRoleMock
	Auth        *service_mocks.AuthMock
	S3          *service_mocks.S3Mock
	Video       *repository_mocks.VideoMock
	VideoAsset  *service_mocks.VideoAssetMock
}

// TestService_Video_Get проверяет выбор точки доступа к видео по таблице статус→ответ
// (§4.4 дизайна эпика).
func TestService_Video_Get(t *testing.T) {
	t.Parallel()

	var (
		testAccountID = uuid.New()
		testGroupID   = uuid.New()
		testUserID    = uuid.New()
		testVideoID   = uuid.New()
		testBucket    = "vilib"
		testHLSToken  = "hls-token"
		testHLSURLTTL = time.Hour
		testOrigURL   = domain.PreflightURL("https://example.com/original")
		testFailure   = "unsupported codec"
	)

	originalAsset := domain.VideoAsset{
		Kind: domain.VideoAssetKindOriginal, Bucket: testBucket, ObjectKey: "videos/x/original",
	}
	masterAsset := domain.VideoAsset{
		Kind: domain.VideoAssetKindHLSMaster, Bucket: testBucket, ObjectKey: "videos/x/hls/master.m3u8",
	}
	variant720 := domain.VideoAsset{Kind: domain.VideoAssetKindHLSVariant, Profile: "720p"}
	variant360 := domain.VideoAsset{Kind: domain.VideoAssetKindHLSVariant, Profile: "360p"}

	grantAccess := func(m videoGetMocks) {
		m.Access.IsCheckAccountActionMock.
			Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionVideoWatch).
			Return(nil)
	}

	tests := []struct {
		name             string
		status           domain.VideoStatus
		isPreferOriginal bool
		assets           []domain.VideoAsset
		failureReason    *string
		setupMocks       func(m videoGetMocks)
		wantKind         domain.VideoAccessKind
		wantProfiles     []string
		wantErr          error
	}{
		{
			name:   "ready without prefer original returns hls with sorted profiles",
			status: domain.VideoStatusReady,
			assets: []domain.VideoAsset{originalAsset, masterAsset, variant720, variant360},
			setupMocks: func(m videoGetMocks) {
				m.Auth.IssueHLSTokenMock.Expect(testVideoID, testHLSURLTTL).Return(testHLSToken, nil)
			},
			wantKind:     domain.VideoAccessKindHLS,
			wantProfiles: []string{"360p", "720p"},
		},
		{
			name:             "ready with prefer original returns original",
			status:           domain.VideoStatusReady,
			isPreferOriginal: true,
			assets:           []domain.VideoAsset{originalAsset, masterAsset},
			setupMocks: func(m videoGetMocks) {
				m.S3.PresignGetObjectMock.
					Expect(minimock.AnyContext, testBucket, originalAsset.ObjectKey, domain.VideoStreamURLTTL).
					Return(testOrigURL, nil)
			},
			wantKind: domain.VideoAccessKindOriginal,
		},
		{
			name:   "ready without master falls back to original",
			status: domain.VideoStatusReady,
			assets: []domain.VideoAsset{originalAsset},
			setupMocks: func(m videoGetMocks) {
				m.S3.PresignGetObjectMock.
					Expect(minimock.AnyContext, testBucket, originalAsset.ObjectKey, domain.VideoStreamURLTTL).
					Return(testOrigURL, nil)
			},
			wantKind: domain.VideoAccessKindOriginal,
		},
		{
			name:   "queued returns original",
			status: domain.VideoStatusQueued,
			assets: []domain.VideoAsset{originalAsset},
			setupMocks: func(m videoGetMocks) {
				m.S3.PresignGetObjectMock.
					Expect(minimock.AnyContext, testBucket, originalAsset.ObjectKey, domain.VideoStreamURLTTL).
					Return(testOrigURL, nil)
			},
			wantKind: domain.VideoAccessKindOriginal,
		},
		{
			name:   "compressing returns original",
			status: domain.VideoStatusCompressing,
			assets: []domain.VideoAsset{originalAsset},
			setupMocks: func(m videoGetMocks) {
				m.S3.PresignGetObjectMock.
					Expect(minimock.AnyContext, testBucket, originalAsset.ObjectKey, domain.VideoStreamURLTTL).
					Return(testOrigURL, nil)
			},
			wantKind: domain.VideoAccessKindOriginal,
		},
		{
			name:          "failed with original returns original",
			status:        domain.VideoStatusFailed,
			assets:        []domain.VideoAsset{originalAsset},
			failureReason: &testFailure,
			setupMocks: func(m videoGetMocks) {
				m.S3.PresignGetObjectMock.
					Expect(minimock.AnyContext, testBucket, originalAsset.ObjectKey, domain.VideoStreamURLTTL).
					Return(testOrigURL, nil)
			},
			wantKind: domain.VideoAccessKindOriginal,
		},
		{
			name:          "failed without original returns conflict",
			status:        domain.VideoStatusFailed,
			assets:        nil,
			failureReason: &testFailure,
			setupMocks:    func(_ videoGetMocks) {},
			wantErr:       service.NewConflictError("video is not available"),
		},
		{
			name:       "uploading returns conflict",
			status:     domain.VideoStatusUploading,
			assets:     nil,
			setupMocks: func(_ videoGetMocks) {},
			wantErr:    service.NewConflictError("video is not available"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mc := minimock.NewController(t)

			m := videoGetMocks{
				Access:      service_mocks.NewAccessMock(mc),
				GroupMember: service_mocks.NewGroupMemberMock(mc),
				GroupRole:   service_mocks.NewGroupRoleMock(mc),
				Auth:        service_mocks.NewAuthMock(mc),
				S3:          service_mocks.NewS3Mock(mc),
				Video:       repository_mocks.NewVideoMock(mc),
				VideoAsset:  service_mocks.NewVideoAssetMock(mc),
			}

			grantAccess(m)

			video := domain.Video{
				ID: testVideoID, GroupID: testGroupID, Status: tt.status, FailureReason: tt.failureReason,
			}
			m.Video.SelectMock.Expect(minimock.AnyContext, testVideoID).Return(&video, nil)
			m.VideoAsset.GetMock.Expect(minimock.AnyContext, testVideoID).Return(tt.assets, nil)

			tt.setupMocks(m)

			svc := service.Service{
				Access:      m.Access,
				GroupMember: m.GroupMember,
				GroupRole:   m.GroupRole,
				Auth:        m.Auth,
				VideoAsset:  m.VideoAsset,
			}

			videoSvc := service.NewVideoService(m.S3, m.Video, &svc, service.VideoServiceConfig{
				Bucket: testBucket,
				Video:  config.VideoConfig{HLSURLTTL: testHLSURLTTL},
			})

			got, err := videoSvc.Get(
				minimock.AnyContext,
				testAccountID,
				testGroupID,
				testUserID,
				testVideoID,
				tt.isPreferOriginal,
			)

			require.Equal(t, tt.wantErr, err)
			if tt.wantErr != nil {
				return
			}

			require.Equal(t, tt.wantKind, got.Kind)
			require.Equal(t, video, got.Video)
			require.Equal(t, tt.wantProfiles, got.Profiles)

			switch tt.wantKind {
			case domain.VideoAccessKindHLS:
				require.Equal(t, testHLSToken, got.HLSToken)
				require.WithinDuration(t, time.Now().Add(testHLSURLTTL), got.ExpiresAt, time.Second)
			case domain.VideoAccessKindOriginal:
				require.Equal(t, testOrigURL, got.URL)
				require.WithinDuration(t, time.Now().Add(domain.VideoStreamURLTTL), got.ExpiresAt, time.Second)
			}
		})
	}

	t.Run("forbidden without permissions", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)

		access := service_mocks.NewAccessMock(mc)
		groupMember := service_mocks.NewGroupMemberMock(mc)

		access.IsCheckAccountActionMock.
			Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionVideoWatch).
			Return(service.ErrForbidden)
		groupMember.GetByUserIDAndGroupIDMock.
			Expect(minimock.AnyContext, testUserID, testGroupID).
			Return(domain.GroupMember{}, service.ErrForbidden)

		svc := service.Service{Access: access, GroupMember: groupMember}
		videoSvc := service.NewVideoService(nil, repository_mocks.NewVideoMock(mc), &svc, service.VideoServiceConfig{})

		_, err := videoSvc.Get(minimock.AnyContext, testAccountID, testGroupID, testUserID, testVideoID, false)

		require.ErrorIs(t, err, service.ErrForbidden)
	})

	t.Run("video belongs to another group is forbidden", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)

		access := service_mocks.NewAccessMock(mc)
		videoRepo := repository_mocks.NewVideoMock(mc)

		access.IsCheckAccountActionMock.
			Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionVideoWatch).
			Return(nil)
		otherGroupVideo := domain.Video{ID: testVideoID, GroupID: uuid.New(), Status: domain.VideoStatusReady}
		videoRepo.SelectMock.Expect(minimock.AnyContext, testVideoID).Return(&otherGroupVideo, nil)

		svc := service.Service{Access: access}
		videoSvc := service.NewVideoService(nil, videoRepo, &svc, service.VideoServiceConfig{})

		_, err := videoSvc.Get(minimock.AnyContext, testAccountID, testGroupID, testUserID, testVideoID, false)

		require.ErrorIs(t, err, service.ErrForbidden)
	})
}

// videoGetAllMocks собирает моки, используемые GetAll.
type videoGetAllMocks struct {
	Access      *service_mocks.AccessMock
	GroupMember *service_mocks.GroupMemberMock
	GroupRole   *service_mocks.GroupRoleMock
	Video       *repository_mocks.VideoMock
	VideoAsset  *service_mocks.VideoAssetMock
}

func newVideoGetAllMocks(mc *minimock.Controller) videoGetAllMocks {
	return videoGetAllMocks{
		Access:      service_mocks.NewAccessMock(mc),
		GroupMember: service_mocks.NewGroupMemberMock(mc),
		GroupRole:   service_mocks.NewGroupRoleMock(mc),
		Video:       repository_mocks.NewVideoMock(mc),
		VideoAsset:  service_mocks.NewVideoAssetMock(mc),
	}
}

func newVideoGetAllService(m videoGetAllMocks) *service.VideoService {
	svc := &service.Service{
		Access:      m.Access,
		GroupMember: m.GroupMember,
		GroupRole:   m.GroupRole,
		VideoAsset:  m.VideoAsset,
	}
	return service.NewVideoService(nil, m.Video, svc, service.VideoServiceConfig{})
}

// TestService_Video_GetAll проверяет сборку списка видео группы (Э1-Т20, §5 дизайна эпика):
// профили и признак обработки собираются из ассетов, причина сбоя (Failure) видна только
// инициатору с правом ManageVideo (аккаунтным или групповым) — иначе остаётся nil даже у видео
// в статусе failed (Э1-Т17).
func TestService_Video_GetAll(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testGroupID := uuid.New()
	testInitiatorID := uuid.New()

	failureClass := domain.VideoFailureClassPermanent
	failureReason := "unsupported codec"

	videoA := domain.Video{
		ID: uuid.New(), GroupID: testGroupID, Name: "a",
		Status: domain.VideoStatusFailed, FailureClass: &failureClass, FailureReason: &failureReason,
	}
	videoB := domain.Video{ID: uuid.New(), GroupID: testGroupID, Name: "b", Status: domain.VideoStatusReady}

	assets := []domain.VideoAsset{
		{VideoID: videoA.ID, Kind: domain.VideoAssetKindOriginal},
		{VideoID: videoB.ID, Kind: domain.VideoAssetKindHLSMaster},
		{VideoID: videoB.ID, Kind: domain.VideoAssetKindHLSVariant, Profile: "720p"},
		{VideoID: videoB.ID, Kind: domain.VideoAssetKindHLSVariant, Profile: "360p"},
	}

	grantWatch := func(m videoGetAllMocks) {
		m.Access.IsCheckAccountActionMock.
			When(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionVideoWatch).
			Then(nil)
	}

	t.Run("forbidden without video watch right", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoGetAllMocks(mc)

		m.Access.IsCheckAccountActionMock.
			When(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionVideoWatch).
			Then(service.ErrForbidden)
		m.GroupMember.GetByUserIDAndGroupIDMock.
			Expect(minimock.AnyContext, testInitiatorID, testGroupID).
			Return(domain.GroupMember{}, service.ErrForbidden)

		videoSvc := newVideoGetAllService(m)

		_, err := videoSvc.GetAll(t.Context(), testAccountID, testGroupID, testInitiatorID)

		require.ErrorIs(t, err, service.ErrForbidden)
	})

	t.Run("hides failure and returns sorted profiles without manage video right", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoGetAllMocks(mc)

		grantWatch(m)
		m.Access.IsCheckAccountActionMock.
			When(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageVideo).
			Then(service.ErrForbidden)
		m.GroupMember.GetByUserIDAndGroupIDMock.
			Expect(minimock.AnyContext, testInitiatorID, testGroupID).
			Return(domain.GroupMember{}, service.ErrForbidden)

		m.Video.SelectByGroupIDMock.Expect(minimock.AnyContext, testGroupID).Return([]domain.Video{videoA, videoB}, nil)
		m.VideoAsset.SelectByVideoIDsMock.
			Expect(minimock.AnyContext, []uuid.UUID{videoA.ID, videoB.ID}).
			Return(assets, nil)

		videoSvc := newVideoGetAllService(m)

		got, err := videoSvc.GetAll(t.Context(), testAccountID, testGroupID, testInitiatorID)

		require.NoError(t, err)
		require.Len(t, got, 2)
		require.Nil(t, got[0].Failure)
		require.False(t, got[0].HasProcessed)
		require.Empty(t, got[0].Profiles)
		require.True(t, got[1].HasProcessed)
		require.Equal(t, []string{"360p", "720p"}, got[1].Profiles)
	})

	t.Run("fills failure for initiator with account manage video right", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoGetAllMocks(mc)

		grantWatch(m)
		m.Access.IsCheckAccountActionMock.
			When(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageVideo).
			Then(nil)

		m.Video.SelectByGroupIDMock.Expect(minimock.AnyContext, testGroupID).Return([]domain.Video{videoA, videoB}, nil)
		m.VideoAsset.SelectByVideoIDsMock.
			Expect(minimock.AnyContext, []uuid.UUID{videoA.ID, videoB.ID}).
			Return(assets, nil)

		videoSvc := newVideoGetAllService(m)

		got, err := videoSvc.GetAll(t.Context(), testAccountID, testGroupID, testInitiatorID)

		require.NoError(t, err)
		require.Len(t, got, 2)
		require.NotNil(t, got[0].Failure)
		require.Equal(t, domain.VideoFailureClassPermanent, got[0].Failure.Class)
		require.Equal(t, "unsupported codec", got[0].Failure.Reason)
		require.Nil(t, got[1].Failure)
	})

	t.Run("fills failure for initiator with group manage video right", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoGetAllMocks(mc)

		grantWatch(m)
		m.Access.IsCheckAccountActionMock.
			When(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageVideo).
			Then(service.ErrForbidden)

		roleID := uuid.New()
		m.GroupMember.GetByUserIDAndGroupIDMock.
			Expect(minimock.AnyContext, testInitiatorID, testGroupID).
			Return(domain.GroupMember{RoleID: roleID}, nil)
		m.GroupRole.GetByIDMock.
			Expect(minimock.AnyContext, roleID).
			Return([]domain.GroupRole{{PermissionMask: domain.PermissionMask(1 << domain.GroupPermissionManageVideo)}}, nil)

		m.Video.SelectByGroupIDMock.Expect(minimock.AnyContext, testGroupID).Return([]domain.Video{videoA}, nil)
		m.VideoAsset.SelectByVideoIDsMock.
			Expect(minimock.AnyContext, []uuid.UUID{videoA.ID}).
			Return(nil, nil)

		videoSvc := newVideoGetAllService(m)

		got, err := videoSvc.GetAll(t.Context(), testAccountID, testGroupID, testInitiatorID)

		require.NoError(t, err)
		require.Len(t, got, 1)
		require.NotNil(t, got[0].Failure)
	})

	t.Run("video asset select error propagates", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoGetAllMocks(mc)

		grantWatch(m)
		m.Access.IsCheckAccountActionMock.
			When(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageVideo).
			Then(nil)

		m.Video.SelectByGroupIDMock.Expect(minimock.AnyContext, testGroupID).Return([]domain.Video{videoA}, nil)
		wantErr := errors.New("assets unavailable")
		m.VideoAsset.SelectByVideoIDsMock.
			Expect(minimock.AnyContext, []uuid.UUID{videoA.ID}).
			Return(nil, wantErr)

		videoSvc := newVideoGetAllService(m)

		_, err := videoSvc.GetAll(t.Context(), testAccountID, testGroupID, testInitiatorID)

		require.ErrorIs(t, err, wantErr)
	})
}

// videoHLSMocks собирает моки, используемые GetHLSMaster/GetHLSPlaylist.
type videoHLSMocks struct {
	Auth       *service_mocks.AuthMock
	S3         *service_mocks.S3Mock
	Video      *repository_mocks.VideoMock
	VideoAsset *service_mocks.VideoAssetMock
}

func newVideoHLSMocks(mc *minimock.Controller) videoHLSMocks {
	return videoHLSMocks{
		Auth:       service_mocks.NewAuthMock(mc),
		S3:         service_mocks.NewS3Mock(mc),
		Video:      repository_mocks.NewVideoMock(mc),
		VideoAsset: service_mocks.NewVideoAssetMock(mc),
	}
}

func newVideoHLSService(m videoHLSMocks, cfg service.VideoServiceConfig) *service.VideoService {
	svc := &service.Service{Auth: m.Auth, VideoAsset: m.VideoAsset}
	return service.NewVideoService(m.S3, m.Video, svc, cfg)
}

func TestService_Video_GetHLSMaster(t *testing.T) {
	t.Parallel()

	testVideoID := uuid.New()
	testBucket := "vilib"
	testToken := "hls-token"
	testMasterKey := "videos/" + testVideoID.String() + "/hls/master.m3u8"
	masterAsset := domain.VideoAsset{Kind: domain.VideoAssetKindHLSMaster, Bucket: testBucket, ObjectKey: testMasterKey}

	t.Run("valid token returns master playlist with rewritten variant uris", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoHLSMocks(mc)

		m.Auth.ParseHLSTokenMock.
			Expect(testToken).
			Return(domain.HLSClaims{Purpose: domain.HLSTokenPurpose, VideoID: testVideoID}, nil)
		m.Video.SelectMock.
			Expect(minimock.AnyContext, testVideoID).
			Return(&domain.Video{ID: testVideoID, Status: domain.VideoStatusReady}, nil)
		m.VideoAsset.GetMock.
			Expect(minimock.AnyContext, testVideoID).
			Return([]domain.VideoAsset{masterAsset}, nil)
		m.S3.GetObjectMock.
			Expect(minimock.AnyContext, testBucket, testMasterKey).
			Return([]byte("#EXTM3U\n720p/playlist.m3u8\n"), nil)

		videoSvc := newVideoHLSService(m, service.VideoServiceConfig{Bucket: testBucket})

		got, err := videoSvc.GetHLSMaster(minimock.AnyContext, testVideoID, testToken)

		require.NoError(t, err)
		require.Equal(t, "#EXTM3U\n720p/playlist.m3u8?token="+testToken+"\n", string(got))
	})

	t.Run("expired or malformed token is unauthorized", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoHLSMocks(mc)

		m.Auth.ParseHLSTokenMock.Expect(testToken).Return(domain.HLSClaims{}, service.ErrUnauthorized)

		videoSvc := newVideoHLSService(m, service.VideoServiceConfig{Bucket: testBucket})

		_, err := videoSvc.GetHLSMaster(minimock.AnyContext, testVideoID, testToken)

		require.ErrorIs(t, err, service.ErrUnauthorized)
	})

	t.Run("token issued for another video is forbidden", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoHLSMocks(mc)

		m.Auth.ParseHLSTokenMock.
			Expect(testToken).
			Return(domain.HLSClaims{Purpose: domain.HLSTokenPurpose, VideoID: uuid.New()}, nil)

		videoSvc := newVideoHLSService(m, service.VideoServiceConfig{Bucket: testBucket})

		_, err := videoSvc.GetHLSMaster(minimock.AnyContext, testVideoID, testToken)

		require.ErrorIs(t, err, service.ErrForbidden)
	})

	t.Run("video not ready returns conflict", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoHLSMocks(mc)

		m.Auth.ParseHLSTokenMock.
			Expect(testToken).
			Return(domain.HLSClaims{Purpose: domain.HLSTokenPurpose, VideoID: testVideoID}, nil)
		m.Video.SelectMock.
			Expect(minimock.AnyContext, testVideoID).
			Return(&domain.Video{ID: testVideoID, Status: domain.VideoStatusCompressing}, nil)

		videoSvc := newVideoHLSService(m, service.VideoServiceConfig{Bucket: testBucket})

		_, err := videoSvc.GetHLSMaster(minimock.AnyContext, testVideoID, testToken)

		require.Equal(t, service.NewConflictError("video is not available"), err)
	})

	t.Run("missing master asset returns not found", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoHLSMocks(mc)

		m.Auth.ParseHLSTokenMock.
			Expect(testToken).
			Return(domain.HLSClaims{Purpose: domain.HLSTokenPurpose, VideoID: testVideoID}, nil)
		m.Video.SelectMock.
			Expect(minimock.AnyContext, testVideoID).
			Return(&domain.Video{ID: testVideoID, Status: domain.VideoStatusReady}, nil)
		m.VideoAsset.GetMock.Expect(minimock.AnyContext, testVideoID).Return(nil, nil)

		videoSvc := newVideoHLSService(m, service.VideoServiceConfig{Bucket: testBucket})

		_, err := videoSvc.GetHLSMaster(minimock.AnyContext, testVideoID, testToken)

		require.ErrorIs(t, err, service.ErrNotFound)
	})
}

func TestService_Video_GetHLSPlaylist(t *testing.T) {
	t.Parallel()

	testVideoID := uuid.New()
	testBucket := "vilib"
	testToken := "hls-token"
	testProfile := domain.VideoProfile("720p")
	testSegmentTTL := time.Hour
	testPlaylistKey := "videos/" + testVideoID.String() + "/hls/720p/playlist.m3u8"
	testSegmentPrefix := "videos/" + testVideoID.String() + "/hls/720p/"
	variantAsset := domain.VideoAsset{
		Kind: domain.VideoAssetKindHLSVariant, Profile: testProfile, Bucket: testBucket, ObjectKey: testPlaylistKey,
	}

	t.Run("valid token returns playlist with signed segment urls", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoHLSMocks(mc)

		m.Auth.ParseHLSTokenMock.
			Expect(testToken).
			Return(domain.HLSClaims{Purpose: domain.HLSTokenPurpose, VideoID: testVideoID}, nil)
		m.Video.SelectMock.
			Expect(minimock.AnyContext, testVideoID).
			Return(&domain.Video{ID: testVideoID, Status: domain.VideoStatusReady}, nil)
		m.VideoAsset.GetMock.
			Expect(minimock.AnyContext, testVideoID).
			Return([]domain.VideoAsset{variantAsset}, nil)
		m.S3.GetObjectMock.
			Expect(minimock.AnyContext, testBucket, testPlaylistKey).
			Return([]byte("#EXTM3U\nseg_00001.ts\n"), nil)
		m.S3.PresignGetObjectMock.
			Expect(minimock.AnyContext, testBucket, testSegmentPrefix+"seg_00001.ts", testSegmentTTL).
			Return(domain.PreflightURL("https://example.com/seg_00001.ts?sig=1"), nil)

		videoSvc := newVideoHLSService(
			m, service.VideoServiceConfig{Bucket: testBucket, Video: config.VideoConfig{HLSSegmentTTL: testSegmentTTL}},
		)

		got, err := videoSvc.GetHLSPlaylist(minimock.AnyContext, testVideoID, testProfile, testToken)

		require.NoError(t, err)
		require.Equal(t, "#EXTM3U\nhttps://example.com/seg_00001.ts?sig=1\n", string(got))
	})

	t.Run("segment presign error propagates", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoHLSMocks(mc)
		presignErr := errors.New("presign failed")

		m.Auth.ParseHLSTokenMock.
			Expect(testToken).
			Return(domain.HLSClaims{Purpose: domain.HLSTokenPurpose, VideoID: testVideoID}, nil)
		m.Video.SelectMock.
			Expect(minimock.AnyContext, testVideoID).
			Return(&domain.Video{ID: testVideoID, Status: domain.VideoStatusReady}, nil)
		m.VideoAsset.GetMock.
			Expect(minimock.AnyContext, testVideoID).
			Return([]domain.VideoAsset{variantAsset}, nil)
		m.S3.GetObjectMock.
			Expect(minimock.AnyContext, testBucket, testPlaylistKey).
			Return([]byte("#EXTM3U\nseg_00001.ts\n"), nil)
		m.S3.PresignGetObjectMock.
			Expect(minimock.AnyContext, testBucket, testSegmentPrefix+"seg_00001.ts", testSegmentTTL).
			Return("", presignErr)

		videoSvc := newVideoHLSService(
			m, service.VideoServiceConfig{Bucket: testBucket, Video: config.VideoConfig{HLSSegmentTTL: testSegmentTTL}},
		)

		_, err := videoSvc.GetHLSPlaylist(minimock.AnyContext, testVideoID, testProfile, testToken)

		require.ErrorIs(t, err, presignErr)
	})

	t.Run("unknown profile returns not found", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoHLSMocks(mc)

		m.Auth.ParseHLSTokenMock.
			Expect(testToken).
			Return(domain.HLSClaims{Purpose: domain.HLSTokenPurpose, VideoID: testVideoID}, nil)
		m.Video.SelectMock.
			Expect(minimock.AnyContext, testVideoID).
			Return(&domain.Video{ID: testVideoID, Status: domain.VideoStatusReady}, nil)
		m.VideoAsset.GetMock.Expect(minimock.AnyContext, testVideoID).Return(nil, nil)

		videoSvc := newVideoHLSService(m, service.VideoServiceConfig{Bucket: testBucket})

		_, err := videoSvc.GetHLSPlaylist(minimock.AnyContext, testVideoID, testProfile, testToken)

		require.ErrorIs(t, err, service.ErrNotFound)
	})

	t.Run("video not ready returns conflict", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoHLSMocks(mc)

		m.Auth.ParseHLSTokenMock.
			Expect(testToken).
			Return(domain.HLSClaims{Purpose: domain.HLSTokenPurpose, VideoID: testVideoID}, nil)
		m.Video.SelectMock.
			Expect(minimock.AnyContext, testVideoID).
			Return(&domain.Video{ID: testVideoID, Status: domain.VideoStatusQueued}, nil)

		videoSvc := newVideoHLSService(m, service.VideoServiceConfig{Bucket: testBucket})

		_, err := videoSvc.GetHLSPlaylist(minimock.AnyContext, testVideoID, testProfile, testToken)

		require.Equal(t, service.NewConflictError("video is not available"), err)
	})
}

// videoDeleteMocks собирает моки, используемые Delete.
type videoDeleteMocks struct {
	Access      *service_mocks.AccessMock
	GroupMember *service_mocks.GroupMemberMock
	S3          *service_mocks.S3Mock
	Video       *repository_mocks.VideoMock
}

func newVideoDeleteMocks(mc *minimock.Controller) videoDeleteMocks {
	return videoDeleteMocks{
		Access:      service_mocks.NewAccessMock(mc),
		GroupMember: service_mocks.NewGroupMemberMock(mc),
		S3:          service_mocks.NewS3Mock(mc),
		Video:       repository_mocks.NewVideoMock(mc),
	}
}

func newVideoDeleteService(
	m videoDeleteMocks,
	cfg service.VideoServiceConfig,
	opts ...service.VideoServiceOption,
) *service.VideoService {
	svc := &service.Service{Access: m.Access, GroupMember: m.GroupMember}
	return service.NewVideoService(m.S3, m.Video, svc, cfg, opts...)
}

// TestService_Video_Delete проверяет удаление видео (Э1-Т21, §7.3 дизайна эпика): проверку
// прав, удаление в БД и best-effort очистку объектов хранилища после коммита транзакции.
func TestService_Video_Delete(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testGroupID := uuid.New()
	testInitiatorID := uuid.New()
	testVideoID := uuid.New()
	testBucket := "vilib"

	t.Run("forbidden - no account or group permission", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoDeleteMocks(mc)
		m.Access.IsCheckAccountActionMock.
			Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageVideo).
			Return(service.ErrForbidden)
		m.GroupMember.GetByUserIDAndGroupIDMock.
			Expect(minimock.AnyContext, testInitiatorID, testGroupID).
			Return(domain.GroupMember{}, errors.New("not a member"))

		videoSvc := newVideoDeleteService(m, service.VideoServiceConfig{Bucket: testBucket})

		err := videoSvc.Delete(t.Context(), testAccountID, testGroupID, testInitiatorID, testVideoID)

		require.ErrorIs(t, err, service.ErrForbidden)
	})

	t.Run("repository error propagates, no cleanup scheduled", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoDeleteMocks(mc)
		m.Access.IsCheckAccountActionMock.
			Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageVideo).
			Return(nil)
		repoErr := errors.New("db unavailable")
		m.Video.DeleteMock.Expect(minimock.AnyContext, testVideoID).Return(repoErr)

		videoSvc := newVideoDeleteService(m, service.VideoServiceConfig{Bucket: testBucket})

		err := videoSvc.Delete(t.Context(), testAccountID, testGroupID, testInitiatorID, testVideoID)

		require.ErrorIs(t, err, repoErr)
	})

	t.Run("success - deletes video and cleans up storage objects after commit", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoDeleteMocks(mc)
		m.Access.IsCheckAccountActionMock.
			Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageVideo).
			Return(nil)
		m.Video.DeleteMock.Expect(minimock.AnyContext, testVideoID).Return(nil)

		var cleanedBucket, cleanedPrefix string
		m.S3.DeleteByPrefixMock.Set(func(_ context.Context, bucket, prefix string) (int, error) {
			cleanedBucket = bucket
			cleanedPrefix = prefix
			return 3, nil
		})

		videoSvc := newVideoDeleteService(m, service.VideoServiceConfig{Bucket: testBucket})

		tx := saga_mocks.NewBobTransactionMock(mc)
		tx.CommitMock.Return(nil)
		tx.RollbackMock.Return(nil)
		tx.RollbackMock.Optional()

		repo := saga_mocks.NewTransactableMock(mc)
		repo.WithTxMock.Return(tx, nil)

		runner := saga.NewSagaRunner(videoSvc, repo)

		err := runner.Run(t.Context(), func(ctx context.Context, svc *service.VideoService) error {
			return svc.Delete(ctx, testAccountID, testGroupID, testInitiatorID, testVideoID)
		})

		require.NoError(t, err)
		require.Equal(t, testBucket, cleanedBucket)
		require.Equal(t, "videos/"+testVideoID.String()+"/", cleanedPrefix)
	})

	t.Run("delete retries exhausted after commit - error logged, Delete already succeeded", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		m := newVideoDeleteMocks(mc)
		m.Access.IsCheckAccountActionMock.
			Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageVideo).
			Return(nil)
		m.Video.DeleteMock.Expect(minimock.AnyContext, testVideoID).Return(nil)

		var attempts int
		m.S3.DeleteByPrefixMock.Set(func(context.Context, string, string) (int, error) {
			attempts++
			return 0, errors.New("s3 unavailable")
		})

		var sleeps []time.Duration
		videoSvc := newVideoDeleteService(
			m,
			service.VideoServiceConfig{Bucket: testBucket},
			service.WithDeleteRetrySleep(func(d time.Duration) { sleeps = append(sleeps, d) }),
		)

		tx := saga_mocks.NewBobTransactionMock(mc)
		tx.CommitMock.Return(nil)
		tx.RollbackMock.Return(nil)
		tx.RollbackMock.Optional()

		repo := saga_mocks.NewTransactableMock(mc)
		repo.WithTxMock.Return(tx, nil)

		runner := saga.NewSagaRunner(videoSvc, repo)

		// Хук выполняется после коммита — ошибка best-effort очистки не должна вернуться
		// вызывающей стороне: удаление видео из БД к этому моменту уже успешно завершено.
		err := runner.Run(t.Context(), func(ctx context.Context, svc *service.VideoService) error {
			return svc.Delete(ctx, testAccountID, testGroupID, testInitiatorID, testVideoID)
		})

		require.NoError(t, err)
		require.Equal(t, 3, attempts)
		require.Equal(t, []time.Duration{200 * time.Millisecond, 400 * time.Millisecond}, sleeps)
	})
}

// TestService_Video_FailTimedOut проверяет watchdog-логику (§8 дизайна эпика, Э1-Т16): три
// вызова UpdateTimedOut с правильными статусами и порогами и best-effort очистку объекта
// оригинала для видео, зависших в uploading, после коммита.
func TestService_Video_FailTimedOut(t *testing.T) {
	t.Parallel()

	testBucket := "vilib"
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	cfg := service.VideoServiceConfig{
		Bucket: testBucket,
		Video: config.VideoConfig{
			UploadTimeout:     2 * time.Hour,
			QueuedTimeout:     time.Hour,
			ProcessingTimeout: 3 * time.Hour,
		},
	}

	t.Run("calls UpdateTimedOut with correct thresholds and cleans up uploading originals", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		videoRepo := repository_mocks.NewVideoMock(mc)

		uploadingID := uuid.New()
		queuedID := uuid.New()
		compressingID := uuid.New()

		videoRepo.UpdateTimedOutMock.Set(func(
			_ context.Context, status domain.VideoStatus, before time.Time, failure domain.VideoFailure,
		) ([]uuid.UUID, error) {
			require.Equal(t, domain.VideoFailureClassTimeout, failure.Class)

			switch status {
			case domain.VideoStatusUploading:
				require.Equal(t, now.Add(-cfg.Video.UploadTimeout), before)
				require.Equal(t, "загрузка не завершена за 2h0m0s", failure.Reason)
				return []uuid.UUID{uploadingID}, nil
			case domain.VideoStatusQueued:
				require.Equal(t, now.Add(-cfg.Video.QueuedTimeout), before)
				require.Equal(t, "не взято в обработку за 1h0m0s", failure.Reason)
				return []uuid.UUID{queuedID}, nil
			case domain.VideoStatusCompressing:
				require.Equal(t, now.Add(-cfg.Video.ProcessingTimeout), before)
				require.Equal(t, "обработка не завершена за 3h0m0s", failure.Reason)
				return []uuid.UUID{compressingID}, nil
			case domain.VideoStatusReady, domain.VideoStatusFailed:
				t.Fatalf("unexpected status %v", status)
				return nil, nil
			}

			t.Fatalf("unexpected status %v", status)
			return nil, nil
		})

		s3Mock := service_mocks.NewS3Mock(mc)
		s3Mock.DeleteObjectMock.
			Expect(minimock.AnyContext, testBucket, domain.VideoOriginalObjectKey(uploadingID)).
			Return(nil)

		videoSvc := service.NewVideoService(s3Mock, videoRepo, &service.Service{}, cfg)

		tx := saga_mocks.NewBobTransactionMock(mc)
		tx.CommitMock.Return(nil)
		tx.RollbackMock.Return(nil)
		tx.RollbackMock.Optional()

		repo := saga_mocks.NewTransactableMock(mc)
		repo.WithTxMock.Return(tx, nil)

		runner := saga.NewSagaRunner(videoSvc, repo)

		var report domain.TimedOutReport
		err := runner.Run(t.Context(), func(ctx context.Context, svc *service.VideoService) error {
			var runErr error
			report, runErr = svc.FailTimedOut(ctx, now)
			return runErr
		})

		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{uploadingID}, report.Uploading)
		require.Equal(t, []uuid.UUID{queuedID}, report.Queued)
		require.Equal(t, []uuid.UUID{compressingID}, report.Compressing)
	})

	t.Run("uploading timeout error propagates, no further statuses checked", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		videoRepo := repository_mocks.NewVideoMock(mc)
		repoErr := errors.New("db unavailable")
		videoRepo.UpdateTimedOutMock.
			Expect(
				minimock.AnyContext,
				domain.VideoStatusUploading,
				now.Add(-cfg.Video.UploadTimeout),
				domain.VideoFailure{Class: domain.VideoFailureClassTimeout, Reason: "загрузка не завершена за 2h0m0s"},
			).
			Return(nil, repoErr)

		videoSvc := service.NewVideoService(service_mocks.NewS3Mock(mc), videoRepo, &service.Service{}, cfg)

		_, err := videoSvc.FailTimedOut(t.Context(), now)

		require.ErrorIs(t, err, repoErr)
	})

	t.Run("no timed out videos - no S3 calls, empty report", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		videoRepo := repository_mocks.NewVideoMock(mc)
		videoRepo.UpdateTimedOutMock.Return(nil, nil)

		videoSvc := service.NewVideoService(service_mocks.NewS3Mock(mc), videoRepo, &service.Service{}, cfg)

		report, err := videoSvc.FailTimedOut(t.Context(), now)

		require.NoError(t, err)
		require.Empty(t, report.Uploading)
		require.Empty(t, report.Queued)
		require.Empty(t, report.Compressing)
	})
}
