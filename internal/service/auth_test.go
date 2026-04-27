package service_test

import (
	"errors"
	"testing"
	"time"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
	"vilib-api/internal/service"
	"vilib-api/internal/service/service_mocks"
	"vilib-api/testutil"

	"github.com/gojuno/minimock/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestService_Auth_Login(t *testing.T) {
	t.Parallel()

	testEmail := testutil.Faker.Person().Contact().Email
	testPassword := testutil.Faker.Person().Name()
	testUserID := uuid.New()
	testAccountID := uuid.New()
	testPasswordHash := testutil.Faker.Hash().MD5()

	var errSomeError = errors.New("some error")

	type args struct {
		email    string
		password string
	}

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.UserMock,
			*service_mocks.AccountMock,
			*service_mocks.AuthMock,
		)
		args    args
		want    string
		wantErr error
	}{
		{
			name: "user not found",
			setupMocks: func(user *service_mocks.UserMock, acc *service_mocks.AccountMock, auth *service_mocks.AuthMock) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return(nil, errSomeError)
			},
			args:    args{testEmail, testPassword},
			want:    "",
			wantErr: errSomeError,
		},
		{
			name: "invalid password",
			setupMocks: func(user *service_mocks.UserMock, acc *service_mocks.AccountMock, auth *service_mocks.AuthMock) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{{ID: testUserID, PasswordHash: testPasswordHash}}, nil)
				auth.ComparePasswordMock.Expect(testPasswordHash, testPassword).
					Return(false)
			},
			args:    args{testEmail, testPassword},
			want:    "",
			wantErr: service.ErrNotFound,
		},
		{
			name: "deactivated user",
			setupMocks: func(user *service_mocks.UserMock, acc *service_mocks.AccountMock, auth *service_mocks.AuthMock) {
				deactivatedAt := time.Now()
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{{ID: testUserID, PasswordHash: testPasswordHash, DeactivatedAt: &deactivatedAt}}, nil)
				auth.ComparePasswordMock.Expect(testPasswordHash, testPassword).
					Return(true)
			},
			args:    args{testEmail, testPassword},
			want:    "",
			wantErr: service.ErrUserDeactivated,
		},
		{
			name: "accounts not found",
			setupMocks: func(user *service_mocks.UserMock, acc *service_mocks.AccountMock, auth *service_mocks.AuthMock) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{{ID: testUserID, PasswordHash: testPasswordHash}}, nil)
				auth.ComparePasswordMock.Expect(testPasswordHash, testPassword).
					Return(true)
				acc.GetByUserEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.Account{}, nil)
			},
			args:    args{testEmail, testPassword},
			want:    "",
			wantErr: service.ErrAccountsNotFound,
		},
		{
			name: "get accounts error",
			setupMocks: func(user *service_mocks.UserMock, acc *service_mocks.AccountMock, auth *service_mocks.AuthMock) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{{ID: testUserID, PasswordHash: testPasswordHash}}, nil)
				auth.ComparePasswordMock.Expect(testPasswordHash, testPassword).
					Return(true)
				acc.GetByUserEmailMock.Expect(minimock.AnyContext, testEmail).
					Return(nil, errSomeError)
			},
			args:    args{testEmail, testPassword},
			want:    "",
			wantErr: nil,
		},
		{
			name: "generate token error",
			setupMocks: func(user *service_mocks.UserMock, acc *service_mocks.AccountMock, auth *service_mocks.AuthMock) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{{ID: testUserID, PasswordHash: testPasswordHash}}, nil)
				auth.ComparePasswordMock.Expect(testPasswordHash, testPassword).
					Return(true)
				acc.GetByUserEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.Account{{ID: testAccountID}}, nil)
				auth.GenerateTokenMock.Expect(testUserID, []uuid.UUID{testAccountID}, testAccountID).
					Return("", errSomeError)
			},
			args:    args{testEmail, testPassword},
			want:    "",
			wantErr: errSomeError,
		},
		{
			name: "success",
			setupMocks: func(user *service_mocks.UserMock, acc *service_mocks.AccountMock, auth *service_mocks.AuthMock) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{{ID: testUserID, PasswordHash: testPasswordHash}}, nil)
				auth.ComparePasswordMock.Expect(testPasswordHash, testPassword).
					Return(true)
				acc.GetByUserEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.Account{{ID: testAccountID}}, nil)
				auth.GenerateTokenMock.Expect(testUserID, []uuid.UUID{testAccountID}, testAccountID).
					Return("test-token", nil)
			},
			args:    args{testEmail, testPassword},
			want:    "test-token",
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, _ *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.User, mockServices.Account, mockServices.Auth)
				},
				func(s *service.Service, r *repository.Repository) {
					cfg := config.AuthConfig{Key: "test-secret-key"}
					srv := service.NewAuthService(cfg, s)

					got, err := srv.Login(t.Context(), tt.args.email, tt.args.password)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}
