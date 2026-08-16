package service_test

import (
	"context"
	"testing"
	"time"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/dberrors"
	"vilib-api/internal/repository/repository_mocks"
	"vilib-api/internal/s3"
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
