package service_test

import (
	"errors"
	"testing"
	"time"
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

func TestService_User_Create(t *testing.T) {
	t.Parallel()

	var (
		testName         = testutil.Faker.Person().FirstName()
		testSurname      = testutil.Faker.Person().LastName()
		testEmail        = testutil.Faker.Person().Contact().Email
		testPasswordHash = testutil.Faker.Hash().MD5()
		testRoleID       = uuid.New()
	)

	var errSomeError = errors.New("some error")

	type args struct {
		name     string
		surname  string
		email    string
		password string
		roleID   uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(*repository_mocks.UserMock)
		args       args
		want       domain.User
		wantErr    error
	}{
		{
			name: "success",
			setupMocks: func(repo *repository_mocks.UserMock) {
				repo.InsertMock.Expect(minimock.AnyContext, testName, testSurname, testPasswordHash, testEmail, testRoleID).
					Return(domain.User{ID: uuid.New(), Email: testEmail}, nil)
			},
			args:    args{testName, testSurname, testEmail, testPasswordHash, testRoleID},
			want:    domain.User{Email: testEmail},
			wantErr: nil,
		},
		{
			name: "insert error",
			setupMocks: func(repo *repository_mocks.UserMock) {
				repo.InsertMock.Expect(minimock.AnyContext, testName, testSurname, testPasswordHash, testEmail, testRoleID).
					Return(domain.User{}, errSomeError)
			},
			args:    args{testName, testSurname, testEmail, testPasswordHash, testRoleID},
			want:    domain.User{},
			wantErr: errSomeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(_ *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockRepos.User)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewUserService(r.User, s)

					got, err := srv.Create(t.Context(), tt.args.name, tt.args.surname, tt.args.email, tt.args.password, tt.args.roleID)

					require.Equal(t, tt.want.Email, got.Email)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

func TestService_User_GetByEmail(t *testing.T) {
	t.Parallel()

	testEmail := testutil.Faker.Person().Contact().Email
	testUserID := uuid.New()

	var errSomeError = errors.New("some error")

	type args struct {
		email string
	}

	tests := []struct {
		name       string
		setupMocks func(*repository_mocks.UserMock)
		args       args
		want       []domain.User
		wantErr    error
	}{
		{
			name: "success",
			setupMocks: func(repo *repository_mocks.UserMock) {
				repo.SelectByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{{ID: testUserID, Email: testEmail}}, nil)
			},
			args:    args{testEmail},
			want:    []domain.User{{ID: testUserID, Email: testEmail}},
			wantErr: nil,
		},
		{
			name: "repo error",
			setupMocks: func(repo *repository_mocks.UserMock) {
				repo.SelectByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return(nil, errSomeError)
			},
			args:    args{testEmail},
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
					tt.setupMocks(mockRepos.User)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewUserService(r.User, s)

					got, err := srv.GetByEmail(t.Context(), tt.args.email)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

func TestService_User_GetByID(t *testing.T) {
	t.Parallel()

	testUserID := uuid.New()

	var errSomeError = errors.New("some error")

	type args struct {
		ids []uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(*repository_mocks.UserMock)
		args       args
		want       []domain.User
		wantErr    error
	}{
		{
			name: "success",
			setupMocks: func(repo *repository_mocks.UserMock) {
				repo.SelectByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID}}, nil)
			},
			args:    args{[]uuid.UUID{testUserID}},
			want:    []domain.User{{ID: testUserID}},
			wantErr: nil,
		},
		{
			name: "repo error",
			setupMocks: func(repo *repository_mocks.UserMock) {
				repo.SelectByIDMock.Expect(minimock.AnyContext, testUserID).
					Return(nil, errSomeError)
			},
			args:    args{[]uuid.UUID{testUserID}},
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
					tt.setupMocks(mockRepos.User)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewUserService(r.User, s)

					got, err := srv.GetByID(t.Context(), tt.args.ids...)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

func TestService_User_Update(t *testing.T) {
	t.Parallel()

	testInitiatorID := uuid.New()
	testAccountID := uuid.New()
	testTargetUserID := uuid.New()
	testRoleID := uuid.New()

	tests := []struct {
		name       string
		setupMocks func(*service_mocks.AccessMock, *repository_mocks.UserMock, *service_mocks.AccountRoleMock)
		args       struct {
			initiatorID  uuid.UUID
			accountID    uuid.UUID
			targetUserID uuid.UUID
			roleID       *uuid.UUID
		}
		want    domain.User
		wantErr error
	}{
		{
			name: "forbidden - no access",
			setupMocks: func(access *service_mocks.AccessMock, _ *repository_mocks.UserMock, _ *service_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.Return(service.ErrForbidden)
			},
			args: struct {
				initiatorID  uuid.UUID
				accountID    uuid.UUID
				targetUserID uuid.UUID
				roleID       *uuid.UUID
			}{testInitiatorID, testAccountID, testTargetUserID, &testRoleID},
			want:    domain.User{},
			wantErr: service.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Access, mockRepos.User, mockServices.AccountRole)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewUserService(r.User, s)

					got, err := srv.Update(t.Context(), tt.args.initiatorID, tt.args.accountID, tt.args.targetUserID, tt.args.roleID)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

func TestService_User_Deactivate(t *testing.T) {
	t.Parallel()

	testInitiatorID := uuid.New()
	testAccountID := uuid.New()
	testTargetID := uuid.New()
	testRoleID := uuid.New()

	now := time.Now()
	activeUser := domain.User{ID: testTargetID, RoleID: testRoleID, DeactivatedAt: nil}
	deactivatedUser := domain.User{ID: testTargetID, RoleID: testRoleID, DeactivatedAt: &now}

	tests := []struct {
		name       string
		setupMocks func(*service_mocks.AccessMock, *repository_mocks.UserMock, *service_mocks.AccountRoleMock)
		wantErr    error
	}{
		{
			name: "success",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserMock, ar *service_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testTargetID).Return([]domain.User{activeUser}, nil)
				ar.GetByIDMock.Expect(minimock.AnyContext, testRoleID).Return([]domain.AccountRole{{ID: testRoleID, IsSystem: false}}, nil)
				repo.DeactivateMock.Expect(minimock.AnyContext, testTargetID).Return(nil)
			},
			wantErr: nil,
		},
		{
			name: "forbidden - no access",
			setupMocks: func(access *service_mocks.AccessMock, _ *repository_mocks.UserMock, _ *service_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.Return(service.ErrForbidden)
			},
			wantErr: service.ErrForbidden,
		},
		{
			name: "conflict - already deactivated",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserMock, _ *service_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testTargetID).Return([]domain.User{deactivatedUser}, nil)
			},
			wantErr: service.ErrUserDeactivated,
		},
		{
			name: "conflict - is owner",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserMock, ar *service_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testTargetID).Return([]domain.User{activeUser}, nil)
				ar.GetByIDMock.Expect(minimock.AnyContext, testRoleID).Return([]domain.AccountRole{{ID: testRoleID, IsSystem: true}}, nil)
			},
			wantErr: service.ErrIsOwner,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Access, mockRepos.User, mockServices.AccountRole)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewUserService(r.User, s)

					err := srv.Deactivate(t.Context(), testInitiatorID, testAccountID, testTargetID)

					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

func TestService_User_Reactivate(t *testing.T) {
	t.Parallel()

	testInitiatorID := uuid.New()
	testAccountID := uuid.New()
	testTargetID := uuid.New()
	testRoleID := uuid.New()
	testDefaultRoleID := uuid.New()

	now := time.Now()
	activeUser := domain.User{ID: testTargetID, RoleID: testRoleID, DeactivatedAt: nil}
	deactivatedUser := domain.User{ID: testTargetID, RoleID: testRoleID, DeactivatedAt: &now}

	tests := []struct {
		name       string
		setupMocks func(*service_mocks.AccessMock, *repository_mocks.UserMock, *service_mocks.AccountRoleMock)
		wantErr    error
	}{
		{
			name: "success",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserMock, ar *service_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testTargetID).Return([]domain.User{deactivatedUser}, nil)
				repo.ReactivateMock.Expect(minimock.AnyContext, testTargetID).Return(nil)
				ar.GetByIDMock.Expect(minimock.AnyContext, testRoleID).Return([]domain.AccountRole{{ID: testRoleID}}, nil)
			},
			wantErr: nil,
		},
		{
			name: "success - role not found, assigns default",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserMock, ar *service_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testTargetID).Return([]domain.User{deactivatedUser}, nil)
				repo.ReactivateMock.Expect(minimock.AnyContext, testTargetID).Return(nil)
				ar.GetByIDMock.Expect(minimock.AnyContext, testRoleID).Return(nil, errors.New("not found"))
				ar.GetDefaultMock.Expect(minimock.AnyContext, testAccountID).Return(domain.AccountRole{ID: testDefaultRoleID}, nil)
				repo.UpdateRoleMock.Expect(minimock.AnyContext, testTargetID, testDefaultRoleID).Return(domain.User{}, nil)
			},
			wantErr: nil,
		},
		{
			name: "forbidden - no access",
			setupMocks: func(access *service_mocks.AccessMock, _ *repository_mocks.UserMock, _ *service_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.Return(service.ErrForbidden)
			},
			wantErr: service.ErrForbidden,
		},
		{
			name: "conflict - already active",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserMock, _ *service_mocks.AccountRoleMock) {
				access.IsCheckAccountActionMock.Return(nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, testTargetID).Return([]domain.User{activeUser}, nil)
			},
			wantErr: service.ErrUserAlreadyActive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Access, mockRepos.User, mockServices.AccountRole)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewUserService(r.User, s)

					err := srv.Reactivate(t.Context(), testInitiatorID, testAccountID, testTargetID)

					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

func TestService_User_ListByAccount(t *testing.T) {
	t.Parallel()

	testInitiatorID := uuid.New()
	testAccountID := uuid.New()

	testUsers := []domain.User{
		{ID: uuid.New(), Email: "user1@example.com"},
	}

	tests := []struct {
		name       string
		status     repository.UserStatus
		setupMocks func(*service_mocks.AccessMock, *repository_mocks.UserMock)
		want       []domain.User
		wantErr    error
	}{
		{
			name:   "success",
			status: repository.UserStatusActive,
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserMock) {
				access.IsCheckAccountActionMock.Return(nil)
				repo.SelectByAccountIDMock.Expect(minimock.AnyContext, testAccountID, repository.UserStatusActive).
					Return(testUsers, nil)
			},
			want:    testUsers,
			wantErr: nil,
		},
		{
			name:   "forbidden",
			status: repository.UserStatusActive,
			setupMocks: func(access *service_mocks.AccessMock, _ *repository_mocks.UserMock) {
				access.IsCheckAccountActionMock.Return(service.ErrForbidden)
			},
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
					tt.setupMocks(mockServices.Access, mockRepos.User)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewUserService(r.User, s)

					got, err := srv.ListByAccount(t.Context(), testInitiatorID, testAccountID, tt.status)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}
