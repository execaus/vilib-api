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

func TestService_GroupMember_Create(t *testing.T) {
	t.Parallel()

	testGroupID := uuid.New()
	testRoleID := uuid.New()
	testUserID := uuid.New()

	var errSomeError = errors.New("some error")

	type args struct {
		groupID uuid.UUID
		roleID  uuid.UUID
		usersID []uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(*repository_mocks.GroupMemberMock)
		args       args
		want       []domain.GroupMember
		wantErr    error
	}{
		{
			name: "success",
			setupMocks: func(repo *repository_mocks.GroupMemberMock) {
				repo.InsertMock.Expect(minimock.AnyContext, testGroupID, testRoleID, testUserID).
					Return([]domain.GroupMember{{GroupID: testGroupID, UserID: testUserID}}, nil)
			},
			args:    args{testGroupID, testRoleID, []uuid.UUID{testUserID}},
			want:    []domain.GroupMember{{GroupID: testGroupID, UserID: testUserID}},
			wantErr: nil,
		},
		{
			name: "insert error",
			setupMocks: func(repo *repository_mocks.GroupMemberMock) {
				repo.InsertMock.Expect(minimock.AnyContext, testGroupID, testRoleID, testUserID).
					Return(nil, errSomeError)
			},
			args:    args{testGroupID, testRoleID, []uuid.UUID{testUserID}},
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
					tt.setupMocks(mockRepos.GroupMember)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewGroupMemberService(r.GroupMember, s)

					got, err := srv.Create(t.Context(), tt.args.groupID, tt.args.roleID, tt.args.usersID...)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}
