package service_test

import (
	"errors"
	"testing"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
	"vilib-api/internal/repository/repository_mocks"
	"vilib-api/internal/service"
	"vilib-api/testutil"

	"github.com/gojuno/minimock/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestService_VideoAsset_Get(t *testing.T) {
	t.Parallel()

	testVideoID := uuid.New()

	var errSomeError = errors.New("some error")

	type args struct {
		videoID uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(*repository_mocks.VideoAssetMock)
		args       args
		want       []domain.VideoAsset
		wantErr    error
	}{
		{
			name: "success",
			setupMocks: func(repo *repository_mocks.VideoAssetMock) {
				repo.SelectMock.Expect(minimock.AnyContext, testVideoID).
					Return([]domain.VideoAsset{{VideoID: testVideoID}}, nil)
			},
			args:    args{testVideoID},
			want:    []domain.VideoAsset{{VideoID: testVideoID}},
			wantErr: nil,
		},
		{
			name: "select error",
			setupMocks: func(repo *repository_mocks.VideoAssetMock) {
				repo.SelectMock.Expect(minimock.AnyContext, testVideoID).
					Return(nil, errSomeError)
			},
			args:    args{testVideoID},
			want:    nil,
			wantErr: errSomeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(_ *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockRepos.VideoAsset)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewVideoAssetService(r.VideoAsset, s)

					got, err := srv.Get(t.Context(), tt.args.videoID)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

func TestService_VideoAsset_Create(t *testing.T) {
	t.Parallel()

	testVideoID := uuid.New()
	testTag := domain.VideoAssetTagOriginal
	testBucketName := "test-bucket"
	testContentType := "video/mp4"
	testBytes := 1024

	var errSomeError = errors.New("some error")

	type args struct {
		videoID     uuid.UUID
		tag         domain.VideoAssetTag
		bucketName  string
		contentType string
		bytes       int
	}

	tests := []struct {
		name       string
		setupMocks func(*repository_mocks.VideoAssetMock)
		args       args
		want       domain.VideoAsset
		wantErr    error
	}{
		{
			name: "create error",
			setupMocks: func(repo *repository_mocks.VideoAssetMock) {
				repo.CreateMock.Expect(minimock.AnyContext, testVideoID, testTag, testBucketName, testContentType, testBytes).
					Return(domain.VideoAsset{}, errSomeError)
			},
			args:    args{testVideoID, testTag, testBucketName, testContentType, testBytes},
			want:    domain.VideoAsset{},
			wantErr: errSomeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(_ *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockRepos.VideoAsset)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewVideoAssetService(r.VideoAsset, s)

					got, err := srv.Create(t.Context(), tt.args.videoID, tt.args.tag, tt.args.bucketName, tt.args.contentType, tt.args.bytes)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}
