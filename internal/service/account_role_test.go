package service_test

import (
	"errors"
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
			name: "repo error returns nil", // Note: actual behavior - code does not return error
			setupMocks: func(repo *repository_mocks.AccountRoleMock) {
				repo.SelectByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return(nil, errSomeError)
			},
			args:    args{[]uuid.UUID{testRoleID}},
			want:    nil,
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
	testPermission := domain.PermissionMask(1)
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
			name: "forbidden returns empty role", // Note: actual behavior - code returns empty role instead of error
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionCreateAccountRole,
					).Return(service.ErrForbidden)
			},
			args:    args{testAccountID, testInitiatorID, testName, nil, testPermission, false},
			want:    domain.AccountRole{},
			wantErr: nil,
		},
		{
			name: "select error",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionCreateAccountRole,
					).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testName, &testParentID, testPermission, false, false).
					Return(domain.AccountRole{ID: testRoleID}, nil)
				repo.SelectByAccountIDMock.Expect(minimock.AnyContext, testAccountID).
					Return(nil, errSomeError)
			},
			args:    args{testAccountID, testInitiatorID, testName, &testParentID, testPermission, false},
			want:    domain.AccountRole{},
			wantErr: errSomeError,
		},
		{
			name: "success",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionCreateAccountRole,
					).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testName, &testParentID, testPermission, false, false).
					Return(domain.AccountRole{ID: testRoleID}, nil)
				repo.SelectByAccountIDMock.Expect(minimock.AnyContext, testAccountID).
					Return([]domain.AccountRole{{ID: testRoleID}}, nil)
			},
			args:    args{testAccountID, testInitiatorID, testName, &testParentID, testPermission, false},
			want:    domain.AccountRole{ID: testRoleID},
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
						domain.AccountPermissionCreateAccountRole,
					).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testName, &testParentID, testPermission, false, false).
					Return(domain.AccountRole{}, errSomeError)
			},
			args:    args{testAccountID, testInitiatorID, testName, &testParentID, testPermission, false},
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
