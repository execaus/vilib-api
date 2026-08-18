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

func TestService_AccountRole_GetDefault(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testRoleID := uuid.New()

	var errSomeError = errors.New("some error")

	type args struct {
		accountID uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(*repository_mocks.AccountRoleMock)
		args       args
		want       domain.AccountRole
		wantErr    error
	}{
		{
			name: "success",
			setupMocks: func(repo *repository_mocks.AccountRoleMock) {
				repo.SelectByAccountIDMock.Expect(minimock.AnyContext, testAccountID).
					Return([]domain.AccountRole{{ID: testRoleID, IsDefault: true}}, nil)
			},
			args:    args{testAccountID},
			want:    domain.AccountRole{ID: testRoleID, IsDefault: true},
			wantErr: nil,
		},
		{
			name: "select error",
			setupMocks: func(repo *repository_mocks.AccountRoleMock) {
				repo.SelectByAccountIDMock.Expect(minimock.AnyContext, testAccountID).
					Return(nil, errSomeError)
			},
			args:    args{testAccountID},
			want:    domain.AccountRole{},
			wantErr: errSomeError,
		},
		{
			name: "default role not found",
			setupMocks: func(repo *repository_mocks.AccountRoleMock) {
				repo.SelectByAccountIDMock.Expect(minimock.AnyContext, testAccountID).
					Return([]domain.AccountRole{{ID: testRoleID, IsDefault: false}}, nil)
			},
			args:    args{testAccountID},
			want:    domain.AccountRole{},
			wantErr: service.ErrDefaultRoleNotFound,
		},
		{
			name: "many default roles",
			setupMocks: func(repo *repository_mocks.AccountRoleMock) {
				repo.SelectByAccountIDMock.Expect(minimock.AnyContext, testAccountID).
					Return([]domain.AccountRole{
						{ID: testRoleID, IsDefault: true},
						{ID: uuid.New(), IsDefault: true},
					}, nil)
			},
			args:    args{testAccountID},
			want:    domain.AccountRole{},
			wantErr: service.ErrDefaultRolesMany,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(_ *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockRepos.AccountRole)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewAccountRoleService(r.AccountRole, s)

					got, err := srv.GetDefault(t.Context(), tt.args.accountID)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

func TestService_AccountRole_GetByID(t *testing.T) {
	t.Parallel()

	testRoleID := uuid.New()

	var errSomeError = errors.New("some error")

	type args struct {
		rolesID []uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(*repository_mocks.AccountRoleMock)
		args       args
		want       []domain.AccountRole
		wantErr    error
	}{
		{
			name: "success",
			setupMocks: func(repo *repository_mocks.AccountRoleMock) {
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{ID: testRoleID}}, nil)
			},
			args:    args{[]uuid.UUID{testRoleID}},
			want:    []domain.AccountRole{{ID: testRoleID}},
			wantErr: nil,
		},
		{
			name: "repo error propagates",
			setupMocks: func(repo *repository_mocks.AccountRoleMock) {
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return(nil, errSomeError)
			},
			args:    args{[]uuid.UUID{testRoleID}},
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
					tt.setupMocks(mockRepos.AccountRole)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewAccountRoleService(r.AccountRole, s)

					got, err := srv.GetByID(t.Context(), tt.args.rolesID...)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

func TestService_AccountRole_CreateSystemAccountOwner(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testRoleID := uuid.New()

	type args struct {
		accountID uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(*repository_mocks.AccountRoleMock)
		args       args
		want       domain.AccountRole
		wantErr    error
	}{
		{
			name: "success",
			setupMocks: func(repo *repository_mocks.AccountRoleMock) {
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, domain.AccountOwnerSystemRoleName, nil, domain.SetBits(0, domain.AccountPermissionOwner), false, true).
					Return(domain.AccountRole{ID: testRoleID}, nil)
				repo.SelectByAccountIDMock.Expect(minimock.AnyContext, testAccountID).
					Return([]domain.AccountRole{{ID: testRoleID}}, nil)
			},
			args:    args{testAccountID},
			want:    domain.AccountRole{ID: testRoleID},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(_ *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockRepos.AccountRole)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewAccountRoleService(r.AccountRole, s)

					got, err := srv.CreateSystemAccountOwner(t.Context(), tt.args.accountID)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

func TestService_AccountRole_Create(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testRoleID := uuid.New()
	testName := testutil.Faker.Person().FirstName()
	// Значение 2 = бит AccountPermissionManageUsers — намеренно не включает бит
	// AccountPermissionOwner (позиция 0), иначе тесты без него столкнулись бы с
	// ErrPermissionOwnerForbidden.
	testPermission := domain.PermissionMask(2)
	testParentID := uuid.New()

	var errSomeError = errors.New("some error")

	type args struct {
		accountID   uuid.UUID
		initiatorID uuid.UUID
		name        string
		parentID    *uuid.UUID
		permission  domain.PermissionMask
		isDefault   bool
	}

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.AccessMock,
			*repository_mocks.AccountRoleMock,
		)
		args    args
		want    domain.AccountRole
		wantErr error
	}{
		{
			name: "forbidden",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageRoles,
					).Return(service.ErrForbidden)
			},
			args:    args{testAccountID, testInitiatorID, testName, nil, testPermission, false},
			want:    domain.AccountRole{},
			wantErr: service.ErrForbidden,
		},
		{
			// Регрессия: Create должен возвращать саму созданную роль (результат Insert), а не
			// первую роль аккаунта из SelectByAccountID (там могла оказаться системная роль
			// owner) — созданная роль отличается от неё именем, маской и accountID.
			name: "success returns the inserted role, not the account's first role",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageRoles,
					).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testName, &testParentID, testPermission, false, false).
					Return(domain.AccountRole{
						ID:             testRoleID,
						Name:           testName,
						AccountID:      testAccountID,
						PermissionMask: testPermission,
						ParentID:       &testParentID,
					}, nil)
			},
			args: args{testAccountID, testInitiatorID, testName, &testParentID, testPermission, false},
			want: domain.AccountRole{
				ID:             testRoleID,
				Name:           testName,
				AccountID:      testAccountID,
				PermissionMask: testPermission,
				ParentID:       &testParentID,
			},
			wantErr: nil,
		},
		{
			name: "insert error",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageRoles,
					).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testName, &testParentID, testPermission, false, false).
					Return(domain.AccountRole{}, errSomeError)
			},
			args:    args{testAccountID, testInitiatorID, testName, &testParentID, testPermission, false},
			want:    domain.AccountRole{},
			wantErr: errSomeError,
		},
		{
			name: "duplicate role name returns conflict",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageRoles,
					).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testName, &testParentID, testPermission, false, false).
					Return(domain.AccountRole{}, dberrors.AccountRoleErrors.ErrUniqueUniqueAccountRole)
			},
			args:    args{testAccountID, testInitiatorID, testName, &testParentID, testPermission, false},
			want:    domain.AccountRole{},
			wantErr: service.ErrAccountRoleNameExists,
		},
		{
			// Бит владельца назначается только системной ролью (CreateSystemAccountOwner),
			// вручную через Create запрещено (§4 дизайна эпика Э2).
			name: "owner permission bit is forbidden",
			setupMocks: func(access *service_mocks.AccessMock, _ *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageRoles,
					).Return(nil)
			},
			args: args{
				testAccountID, testInitiatorID, testName, &testParentID,
				domain.SetBits(testPermission, domain.AccountPermissionOwner), false,
			},
			want:    domain.AccountRole{},
			wantErr: service.ErrPermissionOwnerForbidden,
		},
		{
			// isDefault=true снимает флаг у остальных ролей аккаунта перед вставкой (§4
			// дизайна эпика Э2) — иначе Insert мог бы создать вторую дефолтную роль.
			name: "default role clears previous default before insert",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageRoles,
					).Return(nil)
				repo.ClearDefaultMock.Expect(minimock.AnyContext, testAccountID).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testName, &testParentID, testPermission, true, false).
					Return(domain.AccountRole{ID: testRoleID, IsDefault: true}, nil)
			},
			args:    args{testAccountID, testInitiatorID, testName, &testParentID, testPermission, true},
			want:    domain.AccountRole{ID: testRoleID, IsDefault: true},
			wantErr: nil,
		},
		{
			name: "clear default error propagates",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageRoles,
					).Return(nil)
				repo.ClearDefaultMock.Expect(minimock.AnyContext, testAccountID).Return(errSomeError)
			},
			args:    args{testAccountID, testInitiatorID, testName, &testParentID, testPermission, true},
			want:    domain.AccountRole{},
			wantErr: errSomeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Access, mockRepos.AccountRole)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewAccountRoleService(r.AccountRole, s)

					got, err := srv.Create(
						t.Context(),
						tt.args.accountID,
						tt.args.initiatorID,
						tt.args.name,
						tt.args.parentID,
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

// TestService_AccountRole_GetAll проверяет послабление §3.4 дизайна эпика Э2 (П-7): доступ к
// списку ролей аккаунта получает обладатель ManageRoles или ManageUsers.
func TestService_AccountRole_GetAll(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testRoleID := uuid.New()

	var errSomeError = errors.New("some error")

	tests := []struct {
		name       string
		setupMocks func(*service_mocks.AccessMock, *repository_mocks.AccountRoleMock)
		want       []domain.AccountRole
		wantErr    error
	}{
		{
			name: "manage roles grants access",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageRoles).
					Return(nil)
				repo.SelectByAccountIDMock.Expect(minimock.AnyContext, testAccountID).
					Return([]domain.AccountRole{{ID: testRoleID}}, nil)
			},
			want:    []domain.AccountRole{{ID: testRoleID}},
			wantErr: nil,
		},
		{
			name: "manage users grants access without manage roles",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					When(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageRoles).
					Then(service.ErrForbidden)
				access.IsCheckAccountActionMock.
					When(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageUsers).
					Then(nil)
				repo.SelectByAccountIDMock.Expect(minimock.AnyContext, testAccountID).
					Return([]domain.AccountRole{{ID: testRoleID}}, nil)
			},
			want:    []domain.AccountRole{{ID: testRoleID}},
			wantErr: nil,
		},
		{
			name: "neither permission is forbidden",
			setupMocks: func(access *service_mocks.AccessMock, _ *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					When(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageRoles).
					Then(service.ErrForbidden)
				access.IsCheckAccountActionMock.
					When(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageUsers).
					Then(service.ErrForbidden)
			},
			want:    nil,
			wantErr: service.ErrForbidden,
		},
		{
			name: "non forbidden error from first check propagates immediately",
			setupMocks: func(access *service_mocks.AccessMock, _ *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageRoles).
					Return(errSomeError)
			},
			want:    nil,
			wantErr: errSomeError,
		},
		{
			name: "select error propagates",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageRoles).
					Return(nil)
				repo.SelectByAccountIDMock.Expect(minimock.AnyContext, testAccountID).
					Return(nil, errSomeError)
			},
			want:    nil,
			wantErr: errSomeError,
		},
		{
			// Аккаунт без созданных дополнительных ролей — пустой список, а не 404 (В-43).
			name: "no roles returns empty list without error",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageRoles).
					Return(nil)
				repo.SelectByAccountIDMock.Expect(minimock.AnyContext, testAccountID).
					Return(nil, repository.ErrNotFound)
			},
			want:    []domain.AccountRole{},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Access, mockRepos.AccountRole)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewAccountRoleService(r.AccountRole, s)

					got, err := srv.GetAll(t.Context(), testInitiatorID, testAccountID)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

// TestService_AccountRole_Update проверяет правила полной замены роли аккаунта (§4 дизайна
// эпика Э2): право ManageRoles, роль должна принадлежать accountID, системную роль менять
// нельзя, бит владельца запрещён, единственность дефолтной роли, дубль имени.
func TestService_AccountRole_Update(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testRoleID := uuid.New()
	testName := testutil.Faker.Person().FirstName()
	testMask := domain.PermissionMask(2)
	testParentID := uuid.New()

	var errSomeError = errors.New("some error")

	type args struct {
		initiatorID uuid.UUID
		accountID   uuid.UUID
		roleID      uuid.UUID
		name        string
		parentID    *uuid.UUID
		mask        domain.PermissionMask
		isDefault   bool
	}

	tests := []struct {
		name       string
		setupMocks func(*service_mocks.AccessMock, *repository_mocks.AccountRoleMock)
		args       args
		want       domain.AccountRole
		wantErr    error
	}{
		{
			name: "forbidden",
			setupMocks: func(access *service_mocks.AccessMock, _ *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageRoles).
					Return(service.ErrForbidden)
			},
			args:    args{testInitiatorID, testAccountID, testRoleID, testName, &testParentID, testMask, false},
			want:    domain.AccountRole{},
			wantErr: service.ErrForbidden,
		},
		{
			name: "role from another account is not found",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageRoles).
					Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{ID: testRoleID, AccountID: uuid.New()}}, nil)
			},
			args:    args{testInitiatorID, testAccountID, testRoleID, testName, &testParentID, testMask, false},
			want:    domain.AccountRole{},
			wantErr: service.ErrNotFound,
		},
		{
			name: "role does not exist",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageRoles).
					Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return(nil, repository.ErrNotFound)
			},
			args:    args{testInitiatorID, testAccountID, testRoleID, testName, &testParentID, testMask, false},
			want:    domain.AccountRole{},
			wantErr: service.ErrNotFound,
		},
		{
			name: "system role cannot be edited",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageRoles).
					Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{ID: testRoleID, AccountID: testAccountID, IsSystem: true}}, nil)
			},
			args:    args{testInitiatorID, testAccountID, testRoleID, testName, &testParentID, testMask, false},
			want:    domain.AccountRole{},
			wantErr: service.ErrIsSystemRole,
		},
		{
			name: "owner permission bit is forbidden",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageRoles).
					Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{ID: testRoleID, AccountID: testAccountID}}, nil)
			},
			args: args{
				testInitiatorID, testAccountID, testRoleID, testName, &testParentID,
				domain.SetBits(testMask, domain.AccountPermissionOwner), false,
			},
			want:    domain.AccountRole{},
			wantErr: service.ErrPermissionOwnerForbidden,
		},
		{
			name: "unset default on the only default role is a conflict",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageRoles).
					Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{ID: testRoleID, AccountID: testAccountID, IsDefault: true}}, nil)
			},
			args:    args{testInitiatorID, testAccountID, testRoleID, testName, &testParentID, testMask, false},
			want:    domain.AccountRole{},
			wantErr: service.ErrDefaultRoleRequired,
		},
		{
			name: "default role clears previous default before update",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageRoles).
					Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{ID: testRoleID, AccountID: testAccountID}}, nil)
				repo.ClearDefaultMock.Expect(minimock.AnyContext, testAccountID).Return(nil)
				repo.UpdateMock.Expect(minimock.AnyContext, testRoleID, testName, &testParentID, testMask, true).
					Return(domain.AccountRole{ID: testRoleID, IsDefault: true}, nil)
			},
			args:    args{testInitiatorID, testAccountID, testRoleID, testName, &testParentID, testMask, true},
			want:    domain.AccountRole{ID: testRoleID, IsDefault: true},
			wantErr: nil,
		},
		{
			name: "duplicate role name returns conflict",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageRoles).
					Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{ID: testRoleID, AccountID: testAccountID}}, nil)
				repo.UpdateMock.Expect(minimock.AnyContext, testRoleID, testName, &testParentID, testMask, false).
					Return(domain.AccountRole{}, dberrors.AccountRoleErrors.ErrUniqueUniqueAccountRole)
			},
			args:    args{testInitiatorID, testAccountID, testRoleID, testName, &testParentID, testMask, false},
			want:    domain.AccountRole{},
			wantErr: service.ErrAccountRoleNameExists,
		},
		{
			name: "update error propagates",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageRoles).
					Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{ID: testRoleID, AccountID: testAccountID}}, nil)
				repo.UpdateMock.Expect(minimock.AnyContext, testRoleID, testName, &testParentID, testMask, false).
					Return(domain.AccountRole{}, errSomeError)
			},
			args:    args{testInitiatorID, testAccountID, testRoleID, testName, &testParentID, testMask, false},
			want:    domain.AccountRole{},
			wantErr: errSomeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Access, mockRepos.AccountRole)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewAccountRoleService(r.AccountRole, s)

					got, err := srv.Update(
						t.Context(),
						tt.args.initiatorID, tt.args.accountID, tt.args.roleID,
						tt.args.name, tt.args.parentID, tt.args.mask, tt.args.isDefault,
					)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}
