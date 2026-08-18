package service_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
	"vilib-api/internal/repository/repository_mocks"
	"vilib-api/internal/service"
	"vilib-api/internal/service/service_mocks"
	"vilib-api/testutil"

	"github.com/gojuno/minimock/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestService_Auth_IssueAndParseHLSToken(t *testing.T) {
	t.Parallel()

	testVideoID := uuid.New()
	otherVideoID := uuid.New()
	authSvc := service.NewAuthService(config.AuthConfig{Key: "test-secret-key"}, config.FrontendConfig{}, nil, nil)

	t.Run("valid token round-trips with matching video id", func(t *testing.T) {
		t.Parallel()

		token, err := authSvc.IssueHLSToken(testVideoID, time.Hour)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		claims, err := authSvc.ParseHLSToken(token)
		require.NoError(t, err)
		require.Equal(t, domain.HLSTokenPurpose, claims.Purpose)
		require.Equal(t, testVideoID, claims.VideoID)
		require.NotEqual(t, otherVideoID, claims.VideoID)
	})

	t.Run("expired token is unauthorized", func(t *testing.T) {
		t.Parallel()

		token, err := authSvc.IssueHLSToken(testVideoID, -time.Hour)
		require.NoError(t, err)

		_, err = authSvc.ParseHLSToken(token)
		require.ErrorIs(t, err, service.ErrUnauthorized)
	})

	t.Run("malformed token is unauthorized", func(t *testing.T) {
		t.Parallel()

		_, err := authSvc.ParseHLSToken("not-a-jwt")
		require.ErrorIs(t, err, service.ErrUnauthorized)
	})

	t.Run("token signed with different key is unauthorized", func(t *testing.T) {
		t.Parallel()

		otherAuthSvc := service.NewAuthService(
			config.AuthConfig{Key: "other-secret-key"},
			config.FrontendConfig{},
			nil,
			nil,
		)
		token, err := otherAuthSvc.IssueHLSToken(testVideoID, time.Hour)
		require.NoError(t, err)

		_, err = authSvc.ParseHLSToken(token)
		require.ErrorIs(t, err, service.ErrUnauthorized)
	})

	t.Run("regular auth token is not accepted as hls token", func(t *testing.T) {
		t.Parallel()

		authToken, err := authSvc.GenerateToken(uuid.New(), []uuid.UUID{uuid.New()}, uuid.New())
		require.NoError(t, err)

		_, err = authSvc.ParseHLSToken(authToken)
		require.ErrorIs(t, err, service.ErrUnauthorized)
	})
}

// TestService_Auth_GetClaimsFromToken проверяет разбор значения заголовка Authorization
// (§2.1 дизайна эпика): оба формата "Bearer <jwt>" и голый "<jwt>", просроченный токен
// (ErrTokenExpired, а не nil, nil) и битый/неподписанный токен (ErrTokenInvalid).
func TestService_Auth_GetClaimsFromToken(t *testing.T) {
	t.Parallel()

	authSvc := service.NewAuthService(config.AuthConfig{Key: "test-secret-key"}, config.FrontendConfig{}, nil, nil)
	testUserID := uuid.New()
	testAccountID := uuid.New()

	t.Run("bearer prefixed token is parsed", func(t *testing.T) {
		t.Parallel()

		token, err := authSvc.GenerateToken(testUserID, []uuid.UUID{testAccountID}, testAccountID)
		require.NoError(t, err)

		claims, err := authSvc.GetClaimsFromToken("Bearer " + token)
		require.NoError(t, err)
		require.Equal(t, testUserID, claims.UserID)
	})

	t.Run("bare token without bearer prefix is parsed", func(t *testing.T) {
		t.Parallel()

		token, err := authSvc.GenerateToken(testUserID, []uuid.UUID{testAccountID}, testAccountID)
		require.NoError(t, err)

		claims, err := authSvc.GetClaimsFromToken(token)
		require.NoError(t, err)
		require.Equal(t, testUserID, claims.UserID)
	})

	t.Run("expired token returns ErrTokenExpired, not nil claims and nil error", func(t *testing.T) {
		t.Parallel()

		expiredClaims := domain.AuthClaims{
			UserID:           testUserID,
			CurrentAccountID: testAccountID,
			Accounts:         []uuid.UUID{testAccountID},
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			},
		}
		expiredToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims).
			SignedString([]byte("test-secret-key"))
		require.NoError(t, err)

		claims, err := authSvc.GetClaimsFromToken(expiredToken)
		require.Nil(t, claims)
		require.ErrorIs(t, err, service.ErrTokenExpired)
	})

	t.Run("malformed token returns ErrTokenInvalid", func(t *testing.T) {
		t.Parallel()

		claims, err := authSvc.GetClaimsFromToken("Bearer not-a-jwt")
		require.Nil(t, claims)
		require.ErrorIs(t, err, service.ErrTokenInvalid)
	})

	t.Run("token signed with different key returns ErrTokenInvalid", func(t *testing.T) {
		t.Parallel()

		otherSvc := service.NewAuthService(
			config.AuthConfig{Key: "other-secret-key"},
			config.FrontendConfig{},
			nil,
			nil,
		)
		token, err := otherSvc.GenerateToken(testUserID, []uuid.UUID{testAccountID}, testAccountID)
		require.NoError(t, err)

		claims, err := authSvc.GetClaimsFromToken(token)
		require.Nil(t, claims)
		require.ErrorIs(t, err, service.ErrTokenInvalid)
	})
}

func TestService_Auth_Login(t *testing.T) {
	t.Parallel()

	testEmail := testutil.Faker.Person().Contact().Email
	testPassword := testutil.Faker.Person().Name()
	testUserID := uuid.New()
	testRoleID := uuid.New()
	// testCurrentAccountID — организация роли совпавшей строки; намеренно не совпадает с
	// первым элементом accounts[], чтобы тест ловил регресс на accounts[0] (§2.4 дизайна эпика).
	testCurrentAccountID := uuid.New()
	testOtherAccountID := uuid.New()
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
			*service_mocks.AccountRoleMock,
			*service_mocks.AuthMock,
		)
		args    args
		want    string
		wantErr error
	}{
		{
			name: "get user error propagates",
			setupMocks: func(
				user *service_mocks.UserMock, _ *service_mocks.AccountMock,
				_ *service_mocks.AccountRoleMock, _ *service_mocks.AuthMock,
			) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return(nil, errSomeError)
			},
			args:    args{testEmail, testPassword},
			want:    "",
			wantErr: errSomeError,
		},
		{
			name: "unknown email returns invalid credentials, not not found",
			setupMocks: func(
				user *service_mocks.UserMock, _ *service_mocks.AccountMock,
				_ *service_mocks.AccountRoleMock, _ *service_mocks.AuthMock,
			) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return(nil, repository.ErrNotFound)
			},
			args:    args{testEmail, testPassword},
			want:    "",
			wantErr: service.ErrInvalidCredentials,
		},
		{
			name: "empty user list returns invalid credentials",
			setupMocks: func(
				user *service_mocks.UserMock, _ *service_mocks.AccountMock,
				_ *service_mocks.AccountRoleMock, _ *service_mocks.AuthMock,
			) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{}, nil)
			},
			args:    args{testEmail, testPassword},
			want:    "",
			wantErr: service.ErrInvalidCredentials,
		},
		{
			name: "invalid password",
			setupMocks: func(
				user *service_mocks.UserMock, _ *service_mocks.AccountMock,
				_ *service_mocks.AccountRoleMock, auth *service_mocks.AuthMock,
			) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{{ID: testUserID, PasswordHash: testPasswordHash}}, nil)
				auth.ComparePasswordMock.Expect(testPasswordHash, testPassword).
					Return(false)
			},
			args:    args{testEmail, testPassword},
			want:    "",
			wantErr: service.ErrInvalidCredentials,
		},
		{
			name: "deactivated user",
			setupMocks: func(
				user *service_mocks.UserMock, _ *service_mocks.AccountMock,
				_ *service_mocks.AccountRoleMock, auth *service_mocks.AuthMock,
			) {
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
			name: "prefers active row over deactivated when both rows match the password",
			setupMocks: func(
				user *service_mocks.UserMock, acc *service_mocks.AccountMock,
				role *service_mocks.AccountRoleMock, auth *service_mocks.AuthMock,
			) {
				deactivatedAt := time.Now()
				testDeactivatedUserID := uuid.New()
				testDeactivatedHash := testutil.Faker.Hash().MD5()
				// Деактивированная строка идёт первой в выборке — проверяет, что порядок не влияет
				// на выбор активной строки (Д-6/Д-7 примечание ревью).
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{
						{
							ID: testDeactivatedUserID, RoleID: uuid.New(),
							PasswordHash: testDeactivatedHash, DeactivatedAt: &deactivatedAt,
						},
						{ID: testUserID, RoleID: testRoleID, PasswordHash: testPasswordHash},
					}, nil)
				auth.ComparePasswordMock.When(testDeactivatedHash, testPassword).Then(true)
				auth.ComparePasswordMock.When(testPasswordHash, testPassword).Then(true)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{ID: testRoleID, AccountID: testCurrentAccountID}}, nil)
				acc.GetByUserEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.Account{{ID: testCurrentAccountID}}, nil)
				auth.GenerateTokenMock.Expect(
					testUserID, []uuid.UUID{testCurrentAccountID}, testCurrentAccountID,
				).Return("test-token", nil)
			},
			args:    args{testEmail, testPassword},
			want:    "test-token",
			wantErr: nil,
		},
		{
			name: "get current role error",
			setupMocks: func(
				user *service_mocks.UserMock, _ *service_mocks.AccountMock,
				role *service_mocks.AccountRoleMock, auth *service_mocks.AuthMock,
			) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{{ID: testUserID, RoleID: testRoleID, PasswordHash: testPasswordHash}}, nil)
				auth.ComparePasswordMock.Expect(testPasswordHash, testPassword).
					Return(true)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return(nil, errSomeError)
			},
			args:    args{testEmail, testPassword},
			want:    "",
			wantErr: errSomeError,
		},
		{
			name: "accounts not found",
			setupMocks: func(
				user *service_mocks.UserMock, acc *service_mocks.AccountMock,
				role *service_mocks.AccountRoleMock, auth *service_mocks.AuthMock,
			) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{{ID: testUserID, RoleID: testRoleID, PasswordHash: testPasswordHash}}, nil)
				auth.ComparePasswordMock.Expect(testPasswordHash, testPassword).
					Return(true)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{ID: testRoleID, AccountID: testCurrentAccountID}}, nil)
				acc.GetByUserEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.Account{}, nil)
			},
			args:    args{testEmail, testPassword},
			want:    "",
			wantErr: service.ErrAccountsNotFound,
		},
		{
			name: "get accounts error",
			setupMocks: func(
				user *service_mocks.UserMock, acc *service_mocks.AccountMock,
				role *service_mocks.AccountRoleMock, auth *service_mocks.AuthMock,
			) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{{ID: testUserID, RoleID: testRoleID, PasswordHash: testPasswordHash}}, nil)
				auth.ComparePasswordMock.Expect(testPasswordHash, testPassword).
					Return(true)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{ID: testRoleID, AccountID: testCurrentAccountID}}, nil)
				acc.GetByUserEmailMock.Expect(minimock.AnyContext, testEmail).
					Return(nil, errSomeError)
			},
			args:    args{testEmail, testPassword},
			want:    "",
			wantErr: errSomeError,
		},
		{
			name: "generate token error",
			setupMocks: func(
				user *service_mocks.UserMock, acc *service_mocks.AccountMock,
				role *service_mocks.AccountRoleMock, auth *service_mocks.AuthMock,
			) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{{ID: testUserID, RoleID: testRoleID, PasswordHash: testPasswordHash}}, nil)
				auth.ComparePasswordMock.Expect(testPasswordHash, testPassword).
					Return(true)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{ID: testRoleID, AccountID: testCurrentAccountID}}, nil)
				acc.GetByUserEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.Account{{ID: testOtherAccountID}, {ID: testCurrentAccountID}}, nil)
				auth.GenerateTokenMock.Expect(
					testUserID, []uuid.UUID{testOtherAccountID, testCurrentAccountID}, testCurrentAccountID,
				).Return("", errSomeError)
			},
			args:    args{testEmail, testPassword},
			want:    "",
			wantErr: errSomeError,
		},
		{
			name: "success returns current account id of the matched row, not accounts[0]",
			setupMocks: func(
				user *service_mocks.UserMock, acc *service_mocks.AccountMock,
				role *service_mocks.AccountRoleMock, auth *service_mocks.AuthMock,
			) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{{ID: testUserID, RoleID: testRoleID, PasswordHash: testPasswordHash}}, nil)
				auth.ComparePasswordMock.Expect(testPasswordHash, testPassword).
					Return(true)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{ID: testRoleID, AccountID: testCurrentAccountID}}, nil)
				acc.GetByUserEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.Account{{ID: testOtherAccountID}, {ID: testCurrentAccountID}}, nil)
				auth.GenerateTokenMock.Expect(
					testUserID, []uuid.UUID{testOtherAccountID, testCurrentAccountID}, testCurrentAccountID,
				).Return("test-token", nil)
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
					tt.setupMocks(mockServices.User, mockServices.Account, mockServices.AccountRole, mockServices.Auth)
				},
				func(s *service.Service, r *repository.Repository) {
					cfg := config.AuthConfig{Key: "test-secret-key"}
					srv := service.NewAuthService(cfg, config.FrontendConfig{}, nil, s)

					got, err := srv.Login(t.Context(), tt.args.email, tt.args.password)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

// TestService_Auth_SwitchAccount проверяет переключение текущей организации сессии (§2.4
// дизайна эпика Э2): успех, отсутствие строки пользователя в целевой организации (403
// forbidden), деактивированная строка (403 forbidden.user_deactivated) и проброс ошибок.
func TestService_Auth_SwitchAccount(t *testing.T) {
	t.Parallel()

	testUserID := uuid.New()
	testTargetUserID := uuid.New()
	testAccountID := uuid.New()
	testOtherAccountID := uuid.New()
	testEmail := testutil.Faker.Person().Contact().Email

	var errSomeError = errors.New("some error")

	type args struct {
		userID    uuid.UUID
		accountID uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(*service_mocks.UserMock, *service_mocks.AccountMock, *service_mocks.AuthMock)
		args       args
		want       string
		wantErr    error
	}{
		{
			name: "get current user error",
			setupMocks: func(user *service_mocks.UserMock, _ *service_mocks.AccountMock, _ *service_mocks.AuthMock) {
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return(nil, errSomeError)
			},
			args:    args{testUserID, testAccountID},
			want:    "",
			wantErr: errSomeError,
		},
		{
			name: "not a member of the target account",
			setupMocks: func(user *service_mocks.UserMock, _ *service_mocks.AccountMock, _ *service_mocks.AuthMock) {
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, Email: testEmail}}, nil)
				user.GetByEmailAndAccountIDMock.Expect(minimock.AnyContext, testEmail, testAccountID).
					Return(domain.User{}, repository.ErrNotFound)
			},
			args:    args{testUserID, testAccountID},
			want:    "",
			wantErr: service.ErrNotAccountMember,
		},
		{
			name: "get target user error",
			setupMocks: func(user *service_mocks.UserMock, _ *service_mocks.AccountMock, _ *service_mocks.AuthMock) {
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, Email: testEmail}}, nil)
				user.GetByEmailAndAccountIDMock.Expect(minimock.AnyContext, testEmail, testAccountID).
					Return(domain.User{}, errSomeError)
			},
			args:    args{testUserID, testAccountID},
			want:    "",
			wantErr: errSomeError,
		},
		{
			name: "target row deactivated",
			setupMocks: func(user *service_mocks.UserMock, _ *service_mocks.AccountMock, _ *service_mocks.AuthMock) {
				deactivatedAt := time.Now()
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, Email: testEmail}}, nil)
				user.GetByEmailAndAccountIDMock.Expect(minimock.AnyContext, testEmail, testAccountID).
					Return(domain.User{ID: testTargetUserID, Email: testEmail, DeactivatedAt: &deactivatedAt}, nil)
			},
			args:    args{testUserID, testAccountID},
			want:    "",
			wantErr: service.ErrForbiddenUserDeactivated,
		},
		{
			name: "get accounts error",
			setupMocks: func(user *service_mocks.UserMock, acc *service_mocks.AccountMock, _ *service_mocks.AuthMock) {
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, Email: testEmail}}, nil)
				user.GetByEmailAndAccountIDMock.Expect(minimock.AnyContext, testEmail, testAccountID).
					Return(domain.User{ID: testTargetUserID, Email: testEmail}, nil)
				acc.GetByUserEmailMock.Expect(minimock.AnyContext, testEmail).
					Return(nil, errSomeError)
			},
			args:    args{testUserID, testAccountID},
			want:    "",
			wantErr: errSomeError,
		},
		{
			name: "generate token error",
			setupMocks: func(user *service_mocks.UserMock, acc *service_mocks.AccountMock, auth *service_mocks.AuthMock) {
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, Email: testEmail}}, nil)
				user.GetByEmailAndAccountIDMock.Expect(minimock.AnyContext, testEmail, testAccountID).
					Return(domain.User{ID: testTargetUserID, Email: testEmail}, nil)
				acc.GetByUserEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.Account{{ID: testOtherAccountID}, {ID: testAccountID}}, nil)
				auth.GenerateTokenMock.Expect(
					testTargetUserID, []uuid.UUID{testOtherAccountID, testAccountID}, testAccountID,
				).Return("", errSomeError)
			},
			args:    args{testUserID, testAccountID},
			want:    "",
			wantErr: errSomeError,
		},
		{
			name: "success",
			setupMocks: func(user *service_mocks.UserMock, acc *service_mocks.AccountMock, auth *service_mocks.AuthMock) {
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, Email: testEmail}}, nil)
				user.GetByEmailAndAccountIDMock.Expect(minimock.AnyContext, testEmail, testAccountID).
					Return(domain.User{ID: testTargetUserID, Email: testEmail}, nil)
				acc.GetByUserEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.Account{{ID: testOtherAccountID}, {ID: testAccountID}}, nil)
				auth.GenerateTokenMock.Expect(
					testTargetUserID, []uuid.UUID{testOtherAccountID, testAccountID}, testAccountID,
				).Return("test-token", nil)
			},
			args:    args{testUserID, testAccountID},
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
				func(s *service.Service, _ *repository.Repository) {
					cfg := config.AuthConfig{Key: "test-secret-key"}
					srv := service.NewAuthService(cfg, config.FrontendConfig{}, nil, s)

					got, err := srv.SwitchAccount(t.Context(), tt.args.userID, tt.args.accountID)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

// TestService_Auth_ChangePassword проверяет смену пароля текущей строки пользователя (§6
// дизайна эпика Э2, поправка О-1): неверный старый пароль, новый пароль равен старому, новый
// пароль короче минимальной длины и успешный сценарий.
func TestService_Auth_ChangePassword(t *testing.T) {
	t.Parallel()

	testUserID := uuid.New()
	testOldHash := testutil.Faker.Hash().MD5()
	testNewHash := testutil.Faker.Hash().MD5()

	var errSomeError = errors.New("some error")

	type args struct {
		oldPassword string
		newPassword string
	}

	tests := []struct {
		name       string
		setupMocks func(*service_mocks.UserMock, *service_mocks.AuthMock)
		args       args
		wantErr    error
	}{
		{
			name: "get user error",
			setupMocks: func(user *service_mocks.UserMock, _ *service_mocks.AuthMock) {
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).Return(nil, errSomeError)
			},
			args:    args{oldPassword: "old-password", newPassword: "new-password"},
			wantErr: errSomeError,
		},
		{
			name: "old password invalid",
			setupMocks: func(user *service_mocks.UserMock, auth *service_mocks.AuthMock) {
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, PasswordHash: testOldHash}}, nil)
				auth.ComparePasswordMock.Expect(testOldHash, "wrong-old-password").Return(false)
			},
			args:    args{oldPassword: "wrong-old-password", newPassword: "new-password"},
			wantErr: service.ErrOldPasswordInvalid,
		},
		{
			name: "new password equals old",
			setupMocks: func(user *service_mocks.UserMock, auth *service_mocks.AuthMock) {
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, PasswordHash: testOldHash}}, nil)
				auth.ComparePasswordMock.Expect(testOldHash, "same-password").Return(true)
			},
			args:    args{oldPassword: "same-password", newPassword: "same-password"},
			wantErr: service.ErrPasswordInvalid,
		},
		{
			name: "new password too short",
			setupMocks: func(user *service_mocks.UserMock, auth *service_mocks.AuthMock) {
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, PasswordHash: testOldHash}}, nil)
				auth.ComparePasswordMock.Expect(testOldHash, "old-password").Return(true)
			},
			args:    args{oldPassword: "old-password", newPassword: "short"},
			wantErr: service.ErrPasswordInvalid,
		},
		{
			name: "hash error",
			setupMocks: func(user *service_mocks.UserMock, auth *service_mocks.AuthMock) {
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, PasswordHash: testOldHash}}, nil)
				auth.ComparePasswordMock.When(testOldHash, "old-password").Then(true)
				auth.ComparePasswordMock.When(testOldHash, "new-password").Then(false)
				auth.HashPasswordMock.Expect("new-password").Return("", errSomeError)
			},
			args:    args{oldPassword: "old-password", newPassword: "new-password"},
			wantErr: errSomeError,
		},
		{
			name: "update password hash error",
			setupMocks: func(user *service_mocks.UserMock, auth *service_mocks.AuthMock) {
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, PasswordHash: testOldHash}}, nil)
				auth.ComparePasswordMock.When(testOldHash, "old-password").Then(true)
				auth.ComparePasswordMock.When(testOldHash, "new-password").Then(false)
				auth.HashPasswordMock.Expect("new-password").Return(testNewHash, nil)
				user.UpdatePasswordHashMock.Expect(minimock.AnyContext, testUserID, testNewHash).
					Return(domain.User{}, errSomeError)
			},
			args:    args{oldPassword: "old-password", newPassword: "new-password"},
			wantErr: errSomeError,
		},
		{
			name: "success",
			setupMocks: func(user *service_mocks.UserMock, auth *service_mocks.AuthMock) {
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, PasswordHash: testOldHash}}, nil)
				auth.ComparePasswordMock.When(testOldHash, "old-password").Then(true)
				auth.ComparePasswordMock.When(testOldHash, "new-password").Then(false)
				auth.HashPasswordMock.Expect("new-password").Return(testNewHash, nil)
				user.UpdatePasswordHashMock.Expect(minimock.AnyContext, testUserID, testNewHash).
					Return(domain.User{ID: testUserID, PasswordHash: testNewHash}, nil)
			},
			args:    args{oldPassword: "old-password", newPassword: "new-password"},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, _ *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.User, mockServices.Auth)
				},
				func(s *service.Service, r *repository.Repository) {
					cfg := config.AuthConfig{Key: "test-secret-key"}
					srv := service.NewAuthService(cfg, config.FrontendConfig{}, r.PasswordResetToken, s)

					err := srv.ChangePassword(t.Context(), testUserID, tt.args.oldPassword, tt.args.newPassword)

					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

// TestService_Auth_RequestPasswordReset проверяет запрос сброса пароля (§6 дизайна эпика Э2,
// поправка О-1): email не найден или без активных строк — тихий успех без письма; одна
// организация — одно письмо с одной ссылкой; несколько организаций — письмо со списком ссылок;
// accountID сужает выбор до одной строки.
func TestService_Auth_RequestPasswordReset(t *testing.T) {
	t.Parallel()

	testEmail := testutil.Faker.Person().Contact().Email
	testUserID1 := uuid.New()
	testUserID2 := uuid.New()
	testRoleID1 := uuid.New()
	testRoleID2 := uuid.New()
	testAccountID1 := uuid.New()
	testAccountID2 := uuid.New()
	testAccountName1 := "Организация Один"
	testAccountName2 := "Организация Два"
	deactivatedAt := time.Now()

	var errSomeError = errors.New("some error")

	type args struct {
		email     string
		accountID *uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.UserMock,
			*service_mocks.AccountMock,
			*service_mocks.AccountRoleMock,
			*service_mocks.EmailMock,
			*repository_mocks.PasswordResetTokenMock,
		)
		args    args
		wantErr error
	}{
		{
			name: "email not found is a silent success",
			setupMocks: func(
				user *service_mocks.UserMock, _ *service_mocks.AccountMock,
				_ *service_mocks.AccountRoleMock, _ *service_mocks.EmailMock,
				_ *repository_mocks.PasswordResetTokenMock,
			) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).Return(nil, repository.ErrNotFound)
			},
			args:    args{email: testEmail},
			wantErr: nil,
		},
		{
			name: "get users error propagates",
			setupMocks: func(
				user *service_mocks.UserMock, _ *service_mocks.AccountMock,
				_ *service_mocks.AccountRoleMock, _ *service_mocks.EmailMock,
				_ *repository_mocks.PasswordResetTokenMock,
			) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).Return(nil, errSomeError)
			},
			args:    args{email: testEmail},
			wantErr: errSomeError,
		},
		{
			name: "no active rows is a silent success",
			setupMocks: func(
				user *service_mocks.UserMock, _ *service_mocks.AccountMock,
				_ *service_mocks.AccountRoleMock, _ *service_mocks.EmailMock,
				_ *repository_mocks.PasswordResetTokenMock,
			) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{{ID: testUserID1, Email: testEmail, DeactivatedAt: &deactivatedAt}}, nil)
			},
			args:    args{email: testEmail},
			wantErr: nil,
		},
		{
			name: "accountID does not match any active row is a silent success",
			setupMocks: func(
				user *service_mocks.UserMock, _ *service_mocks.AccountMock,
				role *service_mocks.AccountRoleMock, _ *service_mocks.EmailMock,
				_ *repository_mocks.PasswordResetTokenMock,
			) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{{ID: testUserID1, Email: testEmail, RoleID: testRoleID1}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID1).
					Return([]domain.AccountRole{{ID: testRoleID1, AccountID: testAccountID1}}, nil)
			},
			args:    args{email: testEmail, accountID: &testAccountID2},
			wantErr: nil,
		},
		{
			name: "single active organization sends one link",
			setupMocks: func(
				user *service_mocks.UserMock, acc *service_mocks.AccountMock,
				role *service_mocks.AccountRoleMock, email *service_mocks.EmailMock,
				token *repository_mocks.PasswordResetTokenMock,
			) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{{ID: testUserID1, Email: testEmail, RoleID: testRoleID1}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID1).
					Return([]domain.AccountRole{{ID: testRoleID1, AccountID: testAccountID1}}, nil)
				acc.GetByIDMock.Expect(minimock.AnyContext, testAccountID1).
					Return([]domain.Account{{ID: testAccountID1, Name: testAccountName1}}, nil)
				token.DeleteByEmailMock.Expect(minimock.AnyContext, testEmail).Return(nil)
				token.InsertMock.Set(func(
					_ context.Context, userID uuid.UUID, gotEmail, tokenHash string, expiresAt time.Time,
				) (domain.PasswordResetToken, error) {
					require.Equal(t, testUserID1, userID)
					require.Equal(t, testEmail, gotEmail)
					require.NotEmpty(t, tokenHash)
					require.WithinDuration(t, time.Now().Add(time.Hour), expiresAt, time.Minute)
					return domain.PasswordResetToken{ID: uuid.New(), UserID: userID, Email: gotEmail}, nil
				})
				email.SendPasswordResetMailMock.Set(func(
					_ context.Context, gotEmail string, links []domain.PasswordResetLink, ttl time.Duration,
				) error {
					require.Equal(t, testEmail, gotEmail)
					require.Len(t, links, 1)
					require.Equal(t, testAccountName1, links[0].AccountName)
					require.Contains(t, links[0].URL, "/reset-password?token=")
					require.Equal(t, time.Hour, ttl)
					return nil
				})
			},
			args:    args{email: testEmail},
			wantErr: nil,
		},
		{
			name: "multiple active organizations send a link per organization",
			setupMocks: func(
				user *service_mocks.UserMock, acc *service_mocks.AccountMock,
				role *service_mocks.AccountRoleMock, email *service_mocks.EmailMock,
				token *repository_mocks.PasswordResetTokenMock,
			) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{
						{ID: testUserID1, Email: testEmail, RoleID: testRoleID1},
						{ID: testUserID2, Email: testEmail, RoleID: testRoleID2},
					}, nil)
				role.GetByIDMock.When(minimock.AnyContext, testRoleID1).
					Then([]domain.AccountRole{{ID: testRoleID1, AccountID: testAccountID1}}, nil)
				role.GetByIDMock.When(minimock.AnyContext, testRoleID2).
					Then([]domain.AccountRole{{ID: testRoleID2, AccountID: testAccountID2}}, nil)
				acc.GetByIDMock.When(minimock.AnyContext, testAccountID1).
					Then([]domain.Account{{ID: testAccountID1, Name: testAccountName1}}, nil)
				acc.GetByIDMock.When(minimock.AnyContext, testAccountID2).
					Then([]domain.Account{{ID: testAccountID2, Name: testAccountName2}}, nil)
				token.DeleteByEmailMock.Expect(minimock.AnyContext, testEmail).Return(nil)
				token.InsertMock.Set(func(
					_ context.Context, userID uuid.UUID, gotEmail, tokenHash string, _ time.Time,
				) (domain.PasswordResetToken, error) {
					require.Contains(t, []uuid.UUID{testUserID1, testUserID2}, userID)
					require.NotEmpty(t, tokenHash)
					return domain.PasswordResetToken{ID: uuid.New(), UserID: userID, Email: gotEmail}, nil
				})
				email.SendPasswordResetMailMock.Set(func(
					_ context.Context, gotEmail string, links []domain.PasswordResetLink, _ time.Duration,
				) error {
					require.Equal(t, testEmail, gotEmail)
					require.Len(t, links, 2)
					names := []string{links[0].AccountName, links[1].AccountName}
					require.ElementsMatch(t, []string{testAccountName1, testAccountName2}, names)
					return nil
				})
			},
			args:    args{email: testEmail},
			wantErr: nil,
		},
		{
			name: "send mail error propagates",
			setupMocks: func(
				user *service_mocks.UserMock, acc *service_mocks.AccountMock,
				role *service_mocks.AccountRoleMock, email *service_mocks.EmailMock,
				token *repository_mocks.PasswordResetTokenMock,
			) {
				user.GetByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.User{{ID: testUserID1, Email: testEmail, RoleID: testRoleID1}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID1).
					Return([]domain.AccountRole{{ID: testRoleID1, AccountID: testAccountID1}}, nil)
				acc.GetByIDMock.Expect(minimock.AnyContext, testAccountID1).
					Return([]domain.Account{{ID: testAccountID1, Name: testAccountName1}}, nil)
				token.DeleteByEmailMock.Expect(minimock.AnyContext, testEmail).Return(nil)
				token.InsertMock.Set(func(
					_ context.Context, userID uuid.UUID, gotEmail, _ string, _ time.Time,
				) (domain.PasswordResetToken, error) {
					return domain.PasswordResetToken{ID: uuid.New(), UserID: userID, Email: gotEmail}, nil
				})
				email.SendPasswordResetMailMock.Return(errSomeError)
			},
			args:    args{email: testEmail},
			wantErr: errSomeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(
						mockServices.User, mockServices.Account, mockServices.AccountRole,
						mockServices.Email, mockRepos.PasswordResetToken,
					)
				},
				func(s *service.Service, r *repository.Repository) {
					cfg := config.AuthConfig{Key: "test-secret-key", PasswordResetTTL: time.Hour}
					frontendCfg := config.FrontendConfig{Origin: "http://localhost:5173"}
					srv := service.NewAuthService(cfg, frontendCfg, r.PasswordResetToken, s)

					err := srv.RequestPasswordReset(t.Context(), tt.args.email, tt.args.accountID)

					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

// TestService_Auth_ResetPassword проверяет применение токена сброса пароля (§6 дизайна эпика
// Э2, поправка О-1): несуществующий, использованный и просроченный токен — ErrResetTokenInvalid;
// короткий новый пароль — ErrPasswordInvalid; успешный сценарий помечает токен использованным
// и удаляет остальные токены email.
func TestService_Auth_ResetPassword(t *testing.T) {
	t.Parallel()

	testTokenID := uuid.New()
	testUserID := uuid.New()
	testEmail := testutil.Faker.Person().Contact().Email
	testNewHash := testutil.Faker.Hash().MD5()
	usedAt := time.Now().Add(-time.Minute)

	var errSomeError = errors.New("some error")

	type args struct {
		token       string
		newPassword string
	}

	tests := []struct {
		name       string
		setupMocks func(*service_mocks.UserMock, *service_mocks.AuthMock, *repository_mocks.PasswordResetTokenMock)
		args       args
		wantErr    error
	}{
		{
			name: "password too short",
			setupMocks: func(
				_ *service_mocks.UserMock, _ *service_mocks.AuthMock, _ *repository_mocks.PasswordResetTokenMock,
			) {
			},
			args:    args{token: "raw-token", newPassword: "short"},
			wantErr: service.ErrPasswordInvalid,
		},
		{
			name: "token not found",
			setupMocks: func(
				_ *service_mocks.UserMock, _ *service_mocks.AuthMock, token *repository_mocks.PasswordResetTokenMock,
			) {
				token.SelectByHashMock.Set(func(_ context.Context, _ string) (domain.PasswordResetToken, error) {
					return domain.PasswordResetToken{}, repository.ErrNotFound
				})
			},
			args:    args{token: "raw-token", newPassword: "new-password"},
			wantErr: service.ErrResetTokenInvalid,
		},
		{
			name: "token already used",
			setupMocks: func(
				_ *service_mocks.UserMock, _ *service_mocks.AuthMock, token *repository_mocks.PasswordResetTokenMock,
			) {
				token.SelectByHashMock.Set(func(_ context.Context, _ string) (domain.PasswordResetToken, error) {
					return domain.PasswordResetToken{
						ID: testTokenID, UserID: testUserID, Email: testEmail,
						ExpiresAt: time.Now().Add(time.Hour), UsedAt: &usedAt,
					}, nil
				})
			},
			args:    args{token: "raw-token", newPassword: "new-password"},
			wantErr: service.ErrResetTokenInvalid,
		},
		{
			name: "token expired",
			setupMocks: func(
				_ *service_mocks.UserMock, _ *service_mocks.AuthMock, token *repository_mocks.PasswordResetTokenMock,
			) {
				token.SelectByHashMock.Set(func(_ context.Context, _ string) (domain.PasswordResetToken, error) {
					return domain.PasswordResetToken{
						ID: testTokenID, UserID: testUserID, Email: testEmail,
						ExpiresAt: time.Now().Add(-time.Hour),
					}, nil
				})
			},
			args:    args{token: "raw-token", newPassword: "new-password"},
			wantErr: service.ErrResetTokenInvalid,
		},
		{
			name: "select by hash error propagates",
			setupMocks: func(
				_ *service_mocks.UserMock, _ *service_mocks.AuthMock, token *repository_mocks.PasswordResetTokenMock,
			) {
				token.SelectByHashMock.Set(func(_ context.Context, _ string) (domain.PasswordResetToken, error) {
					return domain.PasswordResetToken{}, errSomeError
				})
			},
			args:    args{token: "raw-token", newPassword: "new-password"},
			wantErr: errSomeError,
		},
		{
			name: "get user error propagates",
			setupMocks: func(
				user *service_mocks.UserMock, _ *service_mocks.AuthMock, token *repository_mocks.PasswordResetTokenMock,
			) {
				token.SelectByHashMock.Set(func(_ context.Context, _ string) (domain.PasswordResetToken, error) {
					return domain.PasswordResetToken{
						ID: testTokenID, UserID: testUserID, Email: testEmail,
						ExpiresAt: time.Now().Add(time.Hour),
					}, nil
				})
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return(nil, errSomeError)
			},
			args:    args{token: "raw-token", newPassword: "new-password"},
			wantErr: errSomeError,
		},
		{
			name: "token belongs to a deactivated row",
			setupMocks: func(
				user *service_mocks.UserMock, _ *service_mocks.AuthMock, token *repository_mocks.PasswordResetTokenMock,
			) {
				deactivatedAt := time.Now()
				token.SelectByHashMock.Set(func(_ context.Context, _ string) (domain.PasswordResetToken, error) {
					return domain.PasswordResetToken{
						ID: testTokenID, UserID: testUserID, Email: testEmail,
						ExpiresAt: time.Now().Add(time.Hour),
					}, nil
				})
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, DeactivatedAt: &deactivatedAt}}, nil)
			},
			args:    args{token: "raw-token", newPassword: "new-password"},
			wantErr: service.ErrResetTokenInvalid,
		},
		{
			name: "success marks token used and deletes the rest",
			setupMocks: func(
				user *service_mocks.UserMock, auth *service_mocks.AuthMock, token *repository_mocks.PasswordResetTokenMock,
			) {
				token.SelectByHashMock.Set(func(_ context.Context, _ string) (domain.PasswordResetToken, error) {
					return domain.PasswordResetToken{
						ID: testTokenID, UserID: testUserID, Email: testEmail,
						ExpiresAt: time.Now().Add(time.Hour),
					}, nil
				})
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID}}, nil)
				auth.HashPasswordMock.Expect("new-password").Return(testNewHash, nil)
				user.UpdatePasswordHashMock.Expect(minimock.AnyContext, testUserID, testNewHash).
					Return(domain.User{ID: testUserID, PasswordHash: testNewHash}, nil)
				token.MarkUsedMock.Expect(minimock.AnyContext, testTokenID).Return(nil)
				token.DeleteByEmailMock.Expect(minimock.AnyContext, testEmail).Return(nil)
			},
			args:    args{token: "raw-token", newPassword: "new-password"},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.User, mockServices.Auth, mockRepos.PasswordResetToken)
				},
				func(s *service.Service, r *repository.Repository) {
					cfg := config.AuthConfig{Key: "test-secret-key", PasswordResetTTL: time.Hour}
					srv := service.NewAuthService(cfg, config.FrontendConfig{}, r.PasswordResetToken, s)

					err := srv.ResetPassword(t.Context(), tt.args.token, tt.args.newPassword)

					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}
