package service_test

import (
	"errors"
	"testing"
	"vilib-api/internal/repository"
	"vilib-api/internal/repository/repository_mocks"
	"vilib-api/internal/service"
	"vilib-api/testutil"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/require"
)

func TestService_Outbox_Publish(t *testing.T) {
	t.Parallel()

	testTopic := "video.original-uploaded"
	testKey := "video-id"
	testPayload := []byte(`{"event_type":"OriginalUploaded"}`)

	var errSomeError = errors.New("some error")

	tests := []struct {
		name       string
		setupMocks func(*repository_mocks.OutboxMock)
		wantErr    error
	}{
		{
			name: "success",
			setupMocks: func(repo *repository_mocks.OutboxMock) {
				repo.InsertMock.Expect(minimock.AnyContext, testTopic, testKey, testPayload).Return(nil)
			},
			wantErr: nil,
		},
		{
			name: "insert error",
			setupMocks: func(repo *repository_mocks.OutboxMock) {
				repo.InsertMock.Expect(minimock.AnyContext, testTopic, testKey, testPayload).Return(errSomeError)
			},
			wantErr: errSomeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(_ *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockRepos.Outbox)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewOutboxService(r.Outbox, s)

					err := srv.Publish(t.Context(), testTopic, testKey, testPayload)

					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}
