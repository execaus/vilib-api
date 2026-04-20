package service_test

import (
	"testing"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository/repository_mocks"
	"vilib-api/internal/service"
	"vilib-api/internal/service/service_mocks"

	"github.com/gojuno/minimock/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestService_Video_GetPreflightUploadURL(t *testing.T) {

	var (
		testAccountID    = uuid.New()
		testGroupID      = uuid.New()
		testUserID       = uuid.New()
		testPreflightURL = domain.PreflightURL("https://example.com/upload")
	)

	testVideo := domain.Video{
		ID:        uuid.New(),
		GroupID:   testGroupID,
		Status:    domain.VideoStatusUploading,
		Name:      "test video",
		Author:    testUserID,
		CreatedAt: time.Now(),
	}

	var errForbidden = service.ErrForbidden

	type args struct {
		accountID uuid.UUID
		groupID   uuid.UUID
		userID    uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(
			*testing.T,
			*service_mocks.AccessMock,
			*service_mocks.GroupMemberMock,
			*service_mocks.GroupRoleMock,
			*service_mocks.S3Mock,
			*repository_mocks.VideoMock,
		)
		args    args
		want    domain.PreflightURL
		wantErr error
	}{
		{
			name: "success",
			setupMocks: func(t *testing.T,
				acc *service_mocks.AccessMock,
				gm *service_mocks.GroupMemberMock,
				gr *service_mocks.GroupRoleMock,
				s3 *service_mocks.S3Mock,
				repo *repository_mocks.VideoMock,
			) {
				groupRoleID := uuid.New()
				acc.IsCheckAccountActionMock.Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionVideoUpload).Return(nil)
				gm.GetByUserIDAndGroupIDMock.Expect(minimock.AnyContext, testUserID, testGroupID).Return(domain.GroupMember{UserID: testUserID, RoleID: groupRoleID}, nil)
				gr.GetByIDMock.Expect(minimock.AnyContext, groupRoleID).Return([]domain.GroupRole{{PermissionMask: domain.PermissionMask(domain.GroupPermissionCreateVideo)}}, nil)
				repo.InsertMock.Expect(minimock.AnyContext, domain.DefaultVideoName, testGroupID, testUserID, domain.VideoStatusUploading).Return(testVideo, nil)
				s3.GetPreflightUploadURLMock.Expect(minimock.AnyContext, domain.VideoBucketOriginal, testVideo.ID, domain.VideoUploadURLTTL).Return(testPreflightURL, nil)
			},
			args: args{
				accountID: testAccountID,
				groupID:   testGroupID,
				userID:    testUserID,
			},
			want: testPreflightURL,
		},
		{
			name: "access denied",
			setupMocks: func(t *testing.T,
				acc *service_mocks.AccessMock,
				gm *service_mocks.GroupMemberMock,
				gr *service_mocks.GroupRoleMock,
				s3 *service_mocks.S3Mock,
				repo *repository_mocks.VideoMock,
			) {
				acc.IsCheckAccountActionMock.Expect(minimock.AnyContext, testAccountID, testUserID, domain.AccountPermissionVideoUpload).Return(errForbidden)
			},
			args: args{
				accountID: testAccountID,
				groupID:   testGroupID,
				userID:    testUserID,
			},
			wantErr: errForbidden,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mc := minimock.NewController(t)
			defer mc.Finish()

			acc := service_mocks.NewAccessMock(mc)
			gm := service_mocks.NewGroupMemberMock(mc)
			gr := service_mocks.NewGroupRoleMock(mc)
			s3 := service_mocks.NewS3Mock(mc)
			repo := repository_mocks.NewVideoMock(mc)

			tt.setupMocks(t, acc, gm, gr, s3, repo)

			svc := service.Service{
				Access:      acc,
				GroupMember: gm,
				GroupRole:   gr,
			}

			videoSvc := service.NewVideoService(s3, repo, &svc)

			got, err := videoSvc.GetPreflightUploadURL(
				minimock.AnyContext,
				tt.args.accountID,
				tt.args.groupID,
				tt.args.userID,
			)

			require.Equal(t, tt.want, got)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestService_Video_Update(t *testing.T) {
	t.Parallel()

	var (
		testVideoID     = uuid.New()
		testGroupID     = uuid.New()
		testInitiatorID = uuid.New()
		testStatus      = domain.VideoStatusReady
	)

	testVideo := domain.Video{
		ID:        testVideoID,
		GroupID:   testGroupID,
		Status:    domain.VideoStatusUploading,
		Name:      "test video",
		CreatedAt: time.Now(),
	}

	testVideoUpdated := domain.Video{
		ID:        testVideoID,
		GroupID:   testGroupID,
		Status:    testStatus,
		Name:      "test video",
		CreatedAt: time.Now(),
	}

	var errNotFound = service.ErrNotFound

	type args struct {
		videoID     uuid.UUID
		initiatorID *uuid.UUID
		status      *domain.VideoStatus
	}

	tests := []struct {
		name       string
		setupMocks func(
			*testing.T,
			*repository_mocks.VideoMock,
		)
		args    args
		want    domain.Video
		wantErr error
	}{
		{
			name: "success without initiator (kafka)",
			setupMocks: func(t *testing.T,
				repo *repository_mocks.VideoMock,
			) {
				repo.SelectMock.Expect(minimock.AnyContext, testVideoID).Return(testVideo, nil)
				repo.UpdateMock.Expect(minimock.AnyContext, testVideoID, &testStatus).Return(testVideoUpdated, nil)
			},
			args: args{
				videoID:     testVideoID,
				initiatorID: nil,
				status:      &testStatus,
			},
			want: testVideoUpdated,
		},
		{
			name: "not found",
			setupMocks: func(t *testing.T,
				repo *repository_mocks.VideoMock,
			) {
				repo.SelectMock.Expect(minimock.AnyContext, testVideoID).Return(domain.Video{}, errNotFound)
			},
			args: args{
				videoID:     testVideoID,
				initiatorID: &testInitiatorID,
				status:      &testStatus,
			},
			wantErr: errNotFound,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mc := minimock.NewController(t)
			defer mc.Finish()

			repo := repository_mocks.NewVideoMock(mc)

			tt.setupMocks(t, repo)

			svc := service.Service{}

			videoSvc := service.NewVideoService(nil, repo, &svc)

			got, err := videoSvc.Update(
				minimock.AnyContext,
				tt.args.videoID,
				tt.args.initiatorID,
				tt.args.status,
			)

			require.Equal(t, tt.want, got)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
