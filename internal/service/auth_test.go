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
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestService_Auth_IssueAndParseHLSToken(t *testing.T) {
	t.Parallel()

	testVideoID := uuid.New()
	otherVideoID := uuid.New()
	authSvc := service.NewAuthService(config.AuthConfig{Key: "test-secret-key"}, nil)

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

		otherAuthSvc := service.NewAuthService(config.AuthConfig{Key: "other-secret-key"}, nil)
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

	authSvc := service.NewAuthService(config.AuthConfig{Key: "test-secret-key"}, nil)
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

		otherSvc := service.NewAuthService(config.AuthConfig{Key: "other-secret-key"}, nil)
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
			name: "user not found",
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
					srv := service.NewAuthService(cfg, s)

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
					srv := service.NewAuthService(cfg, s)

					got, err := srv.SwitchAccount(t.Context(), tt.args.userID, tt.args.accountID)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}
