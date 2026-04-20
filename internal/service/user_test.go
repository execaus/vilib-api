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
	testTargetUserID := uuid.New()
	testRoleID := uuid.New()

	tests := []struct {
		name       string
		setupMocks func(*service_mocks.UserMock)
		args       struct {
			initiatorID  uuid.UUID
			targetUserID uuid.UUID
			roleID       *uuid.UUID
		}
		want    domain.User
		wantErr error
	}{
		{
			name: "not implemented",
			setupMocks: func(user *service_mocks.UserMock) {
			},
			args: struct {
				initiatorID  uuid.UUID
				targetUserID uuid.UUID
				roleID       *uuid.UUID
			}{testInitiatorID, testTargetUserID, &testRoleID},
			want:    domain.User{},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, _ *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.User)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewUserService(r.User, s)

					got, err := srv.Update(t.Context(), tt.args.initiatorID, tt.args.targetUserID, tt.args.roleID)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}
