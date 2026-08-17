package service_test

import (
	"errors"
	"testing"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/dberrors"
	"vilib-api/internal/repository"
	"vilib-api/internal/repository/repository_mocks"
	"vilib-api/internal/service"
	"vilib-api/internal/service/service_mocks"
	"vilib-api/testutil"

	"github.com/gojuno/minimock/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestService_GroupRole_Create(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testRoleID := uuid.New()
	testName := testutil.Faker.Lorem().Word()
	testPermission := domain.PermissionMask(1)

	var errSomeError = errors.New("some error")

	type args struct {
		accountID   uuid.UUID
		initiatorID uuid.UUID
		name        string
		permission  domain.PermissionMask
		isDefault   bool
	}

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.AccessMock,
			*repository_mocks.GroupRoleMock,
		)
		args    args
		want    domain.GroupRole
		wantErr error
	}{
		{
			name: "forbidden",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.GroupRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageRoles,
					).Return(service.ErrForbidden)
			},
			args:    args{testAccountID, testInitiatorID, testName, testPermission, false},
			want:    domain.GroupRole{},
			wantErr: service.ErrForbidden,
		},
		{
			name: "success",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.GroupRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageRoles,
					).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testName, testPermission, false).
					Return(domain.GroupRole{ID: testRoleID}, nil)
			},
			args:    args{testAccountID, testInitiatorID, testName, testPermission, false},
			want:    domain.GroupRole{ID: testRoleID},
			wantErr: nil,
		},
		{
			name: "insert error propagates",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.GroupRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageRoles,
					).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testName, testPermission, false).
					Return(domain.GroupRole{}, errSomeError)
			},
			args:    args{testAccountID, testInitiatorID, testName, testPermission, false},
			want:    domain.GroupRole{},
			wantErr: errSomeError,
		},
		{
			name: "duplicate role name returns conflict",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.GroupRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageRoles,
					).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testName, testPermission, false).
					Return(domain.GroupRole{}, dberrors.GroupRoleErrors.ErrUniqueGroupRolesAccountIdNameKey)
			},
			args:    args{testAccountID, testInitiatorID, testName, testPermission, false},
			want:    domain.GroupRole{},
			wantErr: service.ErrGroupRoleNameExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Access, mockRepos.GroupRole)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewGroupRoleService(r.GroupRole, s)

					got, err := srv.Create(
						t.Context(),
						tt.args.accountID,
						tt.args.initiatorID,
						tt.args.name,
						tt.args.permission,
						tt.args.isDefault,
					)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

// TestService_GroupRole_GetDefault проверяет, что отсутствие роли группы по умолчанию
// превращается в ErrDefaultGroupRoleNotFound (HTTP 409 conflict.default_group_role_missing),
// а не остаётся repository.ErrNotFound (HTTP 404) — §2.2 дизайна эпика.
func TestService_GroupRole_GetDefault(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testRoleID := uuid.New()

	var errSomeError = errors.New("some error")

	tests := []struct {
		name       string
		setupMocks func(*repository_mocks.GroupRoleMock)
		want       domain.GroupRole
		wantErr    error
	}{
		{
			name: "success",
			setupMocks: func(repo *repository_mocks.GroupRoleMock) {
				repo.GetDefaultMock.Expect(minimock.AnyContext, testAccountID).
					Return(domain.GroupRole{ID: testRoleID}, nil)
			},
			want:    domain.GroupRole{ID: testRoleID},
			wantErr: nil,
		},
		{
			name: "no default role returns conflict",
			setupMocks: func(repo *repository_mocks.GroupRoleMock) {
				repo.GetDefaultMock.Expect(minimock.AnyContext, testAccountID).
					Return(domain.GroupRole{}, repository.ErrNotFound)
			},
			want:    domain.GroupRole{},
			wantErr: service.ErrDefaultGroupRoleNotFound,
		},
		{
			name: "select error propagates",
			setupMocks: func(repo *repository_mocks.GroupRoleMock) {
				repo.GetDefaultMock.Expect(minimock.AnyContext, testAccountID).
					Return(domain.GroupRole{}, errSomeError)
			},
			want:    domain.GroupRole{},
			wantErr: errSomeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(_ *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockRepos.GroupRole)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewGroupRoleService(r.GroupRole, s)

					got, err := srv.GetDefault(t.Context(), testAccountID)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}
