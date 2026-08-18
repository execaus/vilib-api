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
						domain.AccountPermissionManageGroups,
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
						domain.AccountPermissionManageGroups,
					).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testName, testPermission, false).
					Return(domain.GroupRole{ID: testRoleID}, nil)
			},
			args:    args{testAccountID, testInitiatorID, testName, testPermission, false},
			want:    domain.GroupRole{ID: testRoleID},
			wantErr: nil,
		},
		{
			// isDefault=true снимает флаг у остальных ролей группы аккаунта перед вставкой
			// (§4 дизайна эпика Э2) — иначе Insert мог бы создать вторую дефолтную роль.
			name: "default role clears previous default before insert",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.GroupRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageGroups,
					).Return(nil)
				repo.ClearDefaultMock.Expect(minimock.AnyContext, testAccountID).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testName, testPermission, true).
					Return(domain.GroupRole{ID: testRoleID, IsDefault: true}, nil)
			},
			args:    args{testAccountID, testInitiatorID, testName, testPermission, true},
			want:    domain.GroupRole{ID: testRoleID, IsDefault: true},
			wantErr: nil,
		},
		{
			name: "clear default error propagates",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.GroupRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageGroups,
					).Return(nil)
				repo.ClearDefaultMock.Expect(minimock.AnyContext, testAccountID).Return(errSomeError)
			},
			args:    args{testAccountID, testInitiatorID, testName, testPermission, true},
			want:    domain.GroupRole{},
			wantErr: errSomeError,
		},
		{
			name: "insert error propagates",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.GroupRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageGroups,
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
						domain.AccountPermissionManageGroups,
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

// TestService_GroupRole_GetAll проверяет послабление §3.4 дизайна эпика Э2 (П-7): доступ к
// списку ролей групп получает любой участник аккаунта (Account.IsHasUser), а не только
// обладатель ManageGroups.
func TestService_GroupRole_GetAll(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testRoleID := uuid.New()

	var errSomeError = errors.New("some error")

	tests := []struct {
		name       string
		setupMocks func(*service_mocks.AccountMock, *repository_mocks.GroupRoleMock)
		want       []domain.GroupRole
		wantErr    error
	}{
		{
			name: "any account member gets the list",
			setupMocks: func(account *service_mocks.AccountMock, repo *repository_mocks.GroupRoleMock) {
				account.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).Return(nil)
				repo.SelectByAccountMock.Expect(minimock.AnyContext, testAccountID).
					Return([]domain.GroupRole{{ID: testRoleID}}, nil)
			},
			want:    []domain.GroupRole{{ID: testRoleID}},
			wantErr: nil,
		},
		{
			name: "not a member of the account is forbidden",
			setupMocks: func(account *service_mocks.AccountMock, _ *repository_mocks.GroupRoleMock) {
				account.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(service.ErrForbidden)
			},
			want:    nil,
			wantErr: service.ErrForbidden,
		},
		{
			name: "select error propagates",
			setupMocks: func(account *service_mocks.AccountMock, repo *repository_mocks.GroupRoleMock) {
				account.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).Return(nil)
				repo.SelectByAccountMock.Expect(minimock.AnyContext, testAccountID).
					Return(nil, errSomeError)
			},
			want:    nil,
			wantErr: errSomeError,
		},
		{
			// Организация без созданных ролей групп — пустой список, а не 404 (В-43).
			name: "no group roles returns empty list without error",
			setupMocks: func(account *service_mocks.AccountMock, repo *repository_mocks.GroupRoleMock) {
				account.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).Return(nil)
				repo.SelectByAccountMock.Expect(minimock.AnyContext, testAccountID).
					Return(nil, repository.ErrNotFound)
			},
			want:    []domain.GroupRole{},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Account, mockRepos.GroupRole)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewGroupRoleService(r.GroupRole, s)

					got, err := srv.GetAll(t.Context(), testInitiatorID, testAccountID)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

// TestService_GroupRole_Update проверяет правила полной замены роли группы (§4 дизайна эпика
// Э2): право ManageGroups, роль должна принадлежать accountID, бит владельца группы разрешён,
// единственность дефолтной роли, дубль имени.
func TestService_GroupRole_Update(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testRoleID := uuid.New()
	testName := testutil.Faker.Lorem().Word()
	testMask := domain.PermissionMask(2)

	var errSomeError = errors.New("some error")

	type args struct {
		initiatorID uuid.UUID
		accountID   uuid.UUID
		roleID      uuid.UUID
		name        string
		mask        domain.PermissionMask
		isDefault   bool
	}

	tests := []struct {
		name       string
		setupMocks func(*service_mocks.AccessMock, *repository_mocks.GroupRoleMock)
		args       args
		want       domain.GroupRole
		wantErr    error
	}{
		{
			name: "forbidden",
			setupMocks: func(access *service_mocks.AccessMock, _ *repository_mocks.GroupRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(service.ErrForbidden)
			},
			args:    args{testInitiatorID, testAccountID, testRoleID, testName, testMask, false},
			want:    domain.GroupRole{},
			wantErr: service.ErrForbidden,
		},
		{
			name: "role from another account is not found",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.GroupRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.GroupRole{{ID: testRoleID, AccountID: uuid.New()}}, nil)
			},
			args:    args{testInitiatorID, testAccountID, testRoleID, testName, testMask, false},
			want:    domain.GroupRole{},
			wantErr: service.ErrNotFound,
		},
		{
			name: "role does not exist",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.GroupRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return(nil, repository.ErrNotFound)
			},
			args:    args{testInitiatorID, testAccountID, testRoleID, testName, testMask, false},
			want:    domain.GroupRole{},
			wantErr: service.ErrNotFound,
		},
		{
			name: "unset default on the only default role is a conflict",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.GroupRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.GroupRole{{ID: testRoleID, AccountID: testAccountID, IsDefault: true}}, nil)
			},
			args:    args{testInitiatorID, testAccountID, testRoleID, testName, testMask, false},
			want:    domain.GroupRole{},
			wantErr: service.ErrDefaultRoleRequired,
		},
		{
			// Бит владельца группы (GroupPermissionOwner) в маске роли группы разрешён — в
			// отличие от ролей аккаунта.
			name: "owner group bit is allowed",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.GroupRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.GroupRole{{ID: testRoleID, AccountID: testAccountID}}, nil)
				ownerMask := domain.SetBits(0, domain.GroupPermissionOwner)
				repo.UpdateMock.Expect(minimock.AnyContext, testRoleID, testName, ownerMask, false).
					Return(domain.GroupRole{ID: testRoleID, PermissionMask: ownerMask}, nil)
			},
			args: args{
				testInitiatorID, testAccountID, testRoleID, testName,
				domain.SetBits(0, domain.GroupPermissionOwner), false,
			},
			want:    domain.GroupRole{ID: testRoleID, PermissionMask: domain.SetBits(0, domain.GroupPermissionOwner)},
			wantErr: nil,
		},
		{
			name: "default role clears previous default before update",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.GroupRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.GroupRole{{ID: testRoleID, AccountID: testAccountID}}, nil)
				repo.ClearDefaultMock.Expect(minimock.AnyContext, testAccountID).Return(nil)
				repo.UpdateMock.Expect(minimock.AnyContext, testRoleID, testName, testMask, true).
					Return(domain.GroupRole{ID: testRoleID, IsDefault: true}, nil)
			},
			args:    args{testInitiatorID, testAccountID, testRoleID, testName, testMask, true},
			want:    domain.GroupRole{ID: testRoleID, IsDefault: true},
			wantErr: nil,
		},
		{
			name: "duplicate role name returns conflict",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.GroupRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.GroupRole{{ID: testRoleID, AccountID: testAccountID}}, nil)
				repo.UpdateMock.Expect(minimock.AnyContext, testRoleID, testName, testMask, false).
					Return(domain.GroupRole{}, dberrors.GroupRoleErrors.ErrUniqueGroupRolesAccountIdNameKey)
			},
			args:    args{testInitiatorID, testAccountID, testRoleID, testName, testMask, false},
			want:    domain.GroupRole{},
			wantErr: service.ErrGroupRoleNameExists,
		},
		{
			name: "update error propagates",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.GroupRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.GroupRole{{ID: testRoleID, AccountID: testAccountID}}, nil)
				repo.UpdateMock.Expect(minimock.AnyContext, testRoleID, testName, testMask, false).
					Return(domain.GroupRole{}, errSomeError)
			},
			args:    args{testInitiatorID, testAccountID, testRoleID, testName, testMask, false},
			want:    domain.GroupRole{},
			wantErr: errSomeError,
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

					got, err := srv.Update(
						t.Context(),
						tt.args.initiatorID, tt.args.accountID, tt.args.roleID,
						tt.args.name, tt.args.mask, tt.args.isDefault,
					)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}
