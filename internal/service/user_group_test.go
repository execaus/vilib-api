package service_test

import (
	"testing"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
	"vilib-api/internal/repository/repository_mocks"
	"vilib-api/internal/service"
	"vilib-api/internal/service/service_mocks"
	"vilib-api/testutil"

	"github.com/gojuno/minimock/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestService_UserGroup_Create(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testGroupID := uuid.New()
	testGroupName := testutil.Faker.Lorem().Word()

	type args struct {
		accountID   uuid.UUID
		initiatorID uuid.UUID
		name        string
	}

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.AccessMock,
			*repository_mocks.UserGroupMock,
		)
		args    args
		want    domain.UserGroup
		wantErr error
	}{
		{
			name: "forbidden",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserGroupMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionCreateUser,
					).Return(service.ErrForbidden)
			},
			args:    args{testAccountID, testInitiatorID, testGroupName},
			want:    domain.UserGroup{},
			wantErr: service.ErrForbidden,
		},
		{
			name: "success",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserGroupMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionCreateUser,
					).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testGroupName).
					Return(domain.UserGroup{ID: testGroupID}, nil)
			},
			args:    args{testAccountID, testInitiatorID, testGroupName},
			want:    domain.UserGroup{ID: testGroupID},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Access, mockRepos.UserGroup)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewUserGroupService(r.UserGroup, s)

					got, err := srv.Create(t.Context(), tt.args.accountID, tt.args.initiatorID, tt.args.name)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

func TestService_UserGroup_AddMembers(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testGroupID := uuid.New()
	testTargetUserID := uuid.New()

	type args struct {
		accountID   uuid.UUID
		initiatorID uuid.UUID
		groupID     uuid.UUID
		targetsID   []uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.AccountMock,
		)
		args    args
		want    []domain.GroupMember
		wantErr error
	}{
		{
			name: "user not in account",
			setupMocks: func(acc *service_mocks.AccountMock) {
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(service.ErrForbidden)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID, []uuid.UUID{testTargetUserID}},
			want:    nil,
			wantErr: service.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Account)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewUserGroupService(r.UserGroup, s)

					got, err := srv.AddMembers(t.Context(), tt.args.accountID, tt.args.initiatorID, tt.args.groupID, tt.args.targetsID...)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}
