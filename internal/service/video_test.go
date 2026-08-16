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
