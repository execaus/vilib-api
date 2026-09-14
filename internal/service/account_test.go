package service_test

import (
	"errors"
	"strings"
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

func TestService_Account_Create(t *testing.T) {
	t.Parallel()

	var (
		testName         = testutil.Faker.Person().FirstName()
		testSurname      = testutil.Faker.Person().LastName()
		testEmail        = testutil.Faker.Person().Contact().Email
		testPassword     = testutil.Faker.Person().Name()
		testPasswordHash = testutil.Faker.Hash().MD5()
		// Название приходит с пробелами по краям: сервис сохраняет обрезанное (A-01 ТЗ).
		testAccountNameRaw = "  ООО «Ромашка»  "
		testAccountName    = "ООО «Ромашка»"
	)

	successAccount := domain.Account{ID: uuid.New()}

	var errSomeError = errors.New("some error")

	type args struct {
		accountName string
		name        string
		surname     string
		email       string
	}

	tests := []struct {
		name       string
		setupMocks func(
			*testing.T,
			*service_mocks.AccountRoleMock,
			*service_mocks.AuthMock,
			*service_mocks.UserMock,
			*service_mocks.EmailMock,
			*repository_mocks.AccountMock,
		)
		args    args
		want    domain.Account
		wantErr error
	}{
		{
			name: "account name too short after trim",
			setupMocks: func(_ *testing.T,
				_ *service_mocks.AccountRoleMock,
				_ *service_mocks.AuthMock,
				_ *service_mocks.UserMock,
				_ *service_mocks.EmailMock,
				_ *repository_mocks.AccountMock,
			) {
			},
			args:    args{"  Я  ", testName, testSurname, testEmail},
			want:    domain.Account{},
			wantErr: service.ErrAccountNameInvalid,
		},
		{
			name: "account name too long",
			setupMocks: func(_ *testing.T,
				_ *service_mocks.AccountRoleMock,
				_ *service_mocks.AuthMock,
				_ *service_mocks.UserMock,
				_ *service_mocks.EmailMock,
				_ *repository_mocks.AccountMock,
			) {
			},
			args:    args{strings.Repeat("я", domain.AccountNameMaxLength+1), testName, testSurname, testEmail},
			want:    domain.Account{},
			wantErr: service.ErrAccountNameInvalid,
		},
		{
			name: "insert error",
			setupMocks: func(_ *testing.T,
				_ *service_mocks.AccountRoleMock,
				_ *service_mocks.AuthMock,
				_ *service_mocks.UserMock,
				_ *service_mocks.EmailMock,
				repo *repository_mocks.AccountMock,
			) {
				repo.InsertMock.Expect(minimock.AnyContext, testAccountName, testEmail).
					Return(domain.Account{}, errSomeError)
			},
			args:    args{testAccountNameRaw, testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "owner role error",
			setupMocks: func(_ *testing.T,
				ar *service_mocks.AccountRoleMock,
				_ *service_mocks.AuthMock,
				_ *service_mocks.UserMock,
				_ *service_mocks.EmailMock,
				repo *repository_mocks.AccountMock,
			) {
				acc := domain.Account{ID: uuid.New()}

				repo.InsertMock.Expect(minimock.AnyContext, testAccountName, testEmail).
					Return(acc, nil)

				ar.CreateSystemAccountOwnerMock.Expect(minimock.AnyContext, acc.ID).
					Return(domain.AccountRole{}, errSomeError)
			},
			args:    args{testAccountNameRaw, testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "generate password error",
			setupMocks: func(_ *testing.T,
				ar *service_mocks.AccountRoleMock,
				auth *service_mocks.AuthMock,
				_ *service_mocks.UserMock,
				_ *service_mocks.EmailMock,
				repo *repository_mocks.AccountMock,
			) {
				acc := domain.Account{ID: uuid.New()}

				repo.InsertMock.Expect(minimock.AnyContext, testAccountName, testEmail).
					Return(acc, nil)

				ar.CreateSystemAccountOwnerMock.Expect(minimock.AnyContext, acc.ID).
					Return(domain.AccountRole{ID: uuid.New()}, nil)

				auth.GeneratePasswordMock.Expect().
					Return("", errSomeError)
			},
			args:    args{testAccountNameRaw, testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "hash password error",
			setupMocks: func(_ *testing.T,
				ar *service_mocks.AccountRoleMock,
				auth *service_mocks.AuthMock,
				_ *service_mocks.UserMock,
				_ *service_mocks.EmailMock,
				repo *repository_mocks.AccountMock,
			) {
				acc := domain.Account{ID: uuid.New()}

				repo.InsertMock.Expect(minimock.AnyContext, testAccountName, testEmail).
					Return(acc, nil)

				ar.CreateSystemAccountOwnerMock.Expect(minimock.AnyContext, acc.ID).
					Return(domain.AccountRole{ID: uuid.New()}, nil)

				auth.GeneratePasswordMock.Expect().
					Return(testPassword, nil)

				auth.HashPasswordMock.Expect(testPassword).
					Return("", errSomeError)
			},
			args:    args{testAccountNameRaw, testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "user create error",
			setupMocks: func(_ *testing.T,
				ar *service_mocks.AccountRoleMock,
				auth *service_mocks.AuthMock,
				user *service_mocks.UserMock,
				_ *service_mocks.EmailMock,
				repo *repository_mocks.AccountMock,
			) {
				acc := domain.Account{ID: uuid.New()}
				role := domain.AccountRole{ID: uuid.New()}

				repo.InsertMock.Expect(minimock.AnyContext, testAccountName, testEmail).
					Return(acc, nil)

				ar.CreateSystemAccountOwnerMock.Expect(minimock.AnyContext, acc.ID).
					Return(role, nil)

				auth.GeneratePasswordMock.Expect().
					Return(testPassword, nil)

				auth.HashPasswordMock.Expect(testPassword).
					Return(testPasswordHash, nil)

				user.CreateMock.Expect(minimock.AnyContext, testName, testSurname, testEmail, testPasswordHash, role.ID).
					Return(domain.User{}, errSomeError)
			},
			args:    args{testAccountNameRaw, testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "email send error",
			setupMocks: func(_ *testing.T,
				ar *service_mocks.AccountRoleMock,
				auth *service_mocks.AuthMock,
				user *service_mocks.UserMock,
				email *service_mocks.EmailMock,
				repo *repository_mocks.AccountMock,
			) {
				acc := domain.Account{ID: uuid.New()}
				role := domain.AccountRole{ID: uuid.New()}

				repo.InsertMock.Expect(minimock.AnyContext, testAccountName, testEmail).
					Return(acc, nil)

				ar.CreateSystemAccountOwnerMock.Expect(minimock.AnyContext, acc.ID).
					Return(role, nil)

				auth.GeneratePasswordMock.Expect().
					Return(testPassword, nil)

				auth.HashPasswordMock.Expect(testPassword).
					Return(testPasswordHash, nil)

				user.CreateMock.Expect(minimock.AnyContext, testName, testSurname, testEmail, testPasswordHash, role.ID).
					Return(domain.User{Email: testEmail}, nil)

				email.SendRegisteredMailMock.Expect(minimock.AnyContext, testEmail, testPassword).
					Return(errSomeError)
			},
			args:    args{testAccountNameRaw, testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "success",
			setupMocks: func(_ *testing.T,
				ar *service_mocks.AccountRoleMock,
				auth *service_mocks.AuthMock,
				user *service_mocks.UserMock,
				email *service_mocks.EmailMock,
				repo *repository_mocks.AccountMock,
			) {
				role := domain.AccountRole{ID: uuid.New()}

				repo.InsertMock.Expect(minimock.AnyContext, testAccountName, testEmail).
					Return(successAccount, nil)

				ar.CreateSystemAccountOwnerMock.Expect(minimock.AnyContext, successAccount.ID).
					Return(role, nil)

				auth.GeneratePasswordMock.Expect().
					Return(testPassword, nil)

				auth.HashPasswordMock.Expect(testPassword).
					Return(testPasswordHash, nil)

				user.CreateMock.Expect(minimock.AnyContext, testName, testSurname, testEmail, testPasswordHash, role.ID).
					Return(domain.User{Email: testEmail}, nil)

				email.SendRegisteredMailMock.Expect(minimock.AnyContext, testEmail, testPassword).
					Return(nil)
			},
			args:    args{testAccountNameRaw, testName, testSurname, testEmail},
			want:    successAccount,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(
						t,
						mockServices.AccountRole,
						mockServices.Auth,
						mockServices.User,
						mockServices.Email,
						mockRepos.Account,
					)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewAccountService(r.Account, s)

					got, err := srv.Create(
						t.Context(), tt.args.accountName, tt.args.name, tt.args.surname, tt.args.email,
					)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

func TestService_Account_CreateUser(t *testing.T) {
	t.Parallel()

	var (
		testName         = testutil.Faker.Person().FirstName()
		testSurname      = testutil.Faker.Person().LastName()
		testEmail        = testutil.Faker.Person().Contact().Email
		testAccountID    = uuid.New()
		testInitiatorID  = uuid.New()
		testPassword     = testutil.Faker.Person().Name()
		testPasswordHash = testutil.Faker.Person().Name()
	)

	var errSomeError = errors.New("some error")

	type args struct {
		accountID   uuid.UUID
		initiatorID uuid.UUID
		name        string
		surname     string
		email       string
	}

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.AccountMock,
			*service_mocks.AuthMock,
			*service_mocks.AccountRoleMock,
			*service_mocks.UserMock,
			*service_mocks.EmailMock,
			*service_mocks.AccessMock,
			*repository_mocks.AccountMock,
		)
		args    args
		want    domain.User
		wantErr error
	}{
		{
			name: "user already exists",
			setupMocks: func(
				_ *service_mocks.AccountMock,
				_ *service_mocks.AuthMock,
				_ *service_mocks.AccountRoleMock,
				user *service_mocks.UserMock,
				_ *service_mocks.EmailMock,
				access *service_mocks.AccessMock,
				_ *repository_mocks.AccountMock,
			) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageUsers,
					).Return(nil)
				user.GetByEmailAndAccountIDMock.Expect(minimock.AnyContext, testEmail, testAccountID).
					Return(domain.User{Email: testEmail}, nil)
			},
			args:    args{testAccountID, testInitiatorID, testName, testSurname, testEmail},
			wantErr: service.ErrAccountUserExists,
		},
		{
			name: "user existence check error",
			setupMocks: func(
				_ *service_mocks.AccountMock,
				_ *service_mocks.AuthMock,
				_ *service_mocks.AccountRoleMock,
				user *service_mocks.UserMock,
				_ *service_mocks.EmailMock,
				access *service_mocks.AccessMock,
				_ *repository_mocks.AccountMock,
			) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageUsers,
					).Return(nil)
				user.GetByEmailAndAccountIDMock.Expect(minimock.AnyContext, testEmail, testAccountID).
					Return(domain.User{}, errSomeError)
			},
			args:    args{testAccountID, testInitiatorID, testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "generate testPassword error",
			setupMocks: func(
				_ *service_mocks.AccountMock,
				auth *service_mocks.AuthMock,
				_ *service_mocks.AccountRoleMock,
				user *service_mocks.UserMock,
				_ *service_mocks.EmailMock,
				access *service_mocks.AccessMock,
				_ *repository_mocks.AccountMock,
			) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageUsers,
					).Return(nil)
				user.GetByEmailAndAccountIDMock.Expect(minimock.AnyContext, testEmail, testAccountID).
					Return(domain.User{}, repository.ErrNotFound)
				auth.GeneratePasswordMock.Expect().
					Return("", errSomeError)
			},
			args:    args{testAccountID, testInitiatorID, testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "hash password error",
			setupMocks: func(
				_ *service_mocks.AccountMock,
				auth *service_mocks.AuthMock,
				_ *service_mocks.AccountRoleMock,
				user *service_mocks.UserMock,
				_ *service_mocks.EmailMock,
				access *service_mocks.AccessMock,
				_ *repository_mocks.AccountMock,
			) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageUsers,
					).Return(nil)
				user.GetByEmailAndAccountIDMock.Expect(minimock.AnyContext, testEmail, testAccountID).
					Return(domain.User{}, repository.ErrNotFound)
				auth.GeneratePasswordMock.Expect().
					Return(testPassword, nil)
				auth.HashPasswordMock.Expect(testPassword).
					Return("", errSomeError)
			},
			args:    args{testAccountID, testInitiatorID, testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "get default role error",
			setupMocks: func(
				_ *service_mocks.AccountMock,
				auth *service_mocks.AuthMock,
				ar *service_mocks.AccountRoleMock,
				user *service_mocks.UserMock,
				_ *service_mocks.EmailMock,
				access *service_mocks.AccessMock,
				_ *repository_mocks.AccountMock,
			) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageUsers,
					).Return(nil)
				user.GetByEmailAndAccountIDMock.Expect(minimock.AnyContext, testEmail, testAccountID).
					Return(domain.User{}, repository.ErrNotFound)
				auth.GeneratePasswordMock.Expect().
					Return(testPassword, nil)
				auth.HashPasswordMock.Expect(testPassword).
					Return(testPasswordHash, nil)
				ar.GetDefaultMock.Expect(minimock.AnyContext, testAccountID).
					Return(domain.AccountRole{}, errSomeError)
			},
			args:    args{testAccountID, testInitiatorID, testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "user create error",
			setupMocks: func(
				_ *service_mocks.AccountMock,
				auth *service_mocks.AuthMock,
				ar *service_mocks.AccountRoleMock,
				user *service_mocks.UserMock,
				_ *service_mocks.EmailMock,
				access *service_mocks.AccessMock,
				_ *repository_mocks.AccountMock,
			) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageUsers,
					).Return(nil)

				role := domain.AccountRole{ID: uuid.New()}

				user.GetByEmailAndAccountIDMock.Expect(minimock.AnyContext, testEmail, testAccountID).
					Return(domain.User{}, repository.ErrNotFound)
				auth.GeneratePasswordMock.Expect().
					Return(testPassword, nil)
				auth.HashPasswordMock.Expect(testPassword).
					Return(testPasswordHash, nil)
				ar.GetDefaultMock.Expect(minimock.AnyContext, testAccountID).
					Return(role, nil)
				user.CreateMock.Expect(minimock.AnyContext, testName, testSurname, testEmail, testPasswordHash, role.ID).
					Return(domain.User{}, errSomeError)
			},
			args:    args{testAccountID, testInitiatorID, testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "email send error",
			setupMocks: func(
				_ *service_mocks.AccountMock,
				auth *service_mocks.AuthMock,
				ar *service_mocks.AccountRoleMock,
				user *service_mocks.UserMock,
				email *service_mocks.EmailMock,
				access *service_mocks.AccessMock,
				_ *repository_mocks.AccountMock,
			) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageUsers,
					).Return(nil)

				role := domain.AccountRole{ID: uuid.New()}

				user.GetByEmailAndAccountIDMock.Expect(minimock.AnyContext, testEmail, testAccountID).
					Return(domain.User{}, repository.ErrNotFound)
				auth.GeneratePasswordMock.Expect().
					Return(testPassword, nil)
				auth.HashPasswordMock.Expect(testPassword).
					Return(testPasswordHash, nil)
				ar.GetDefaultMock.Expect(minimock.AnyContext, testAccountID).
					Return(role, nil)
				user.CreateMock.Expect(minimock.AnyContext, testName, testSurname, testEmail, testPasswordHash, role.ID).
					Return(domain.User{Email: testEmail}, nil)
				email.SendCreateUserEmailMock.Expect(minimock.AnyContext, testEmail, testPassword).
					Return(errSomeError)
			},
			args:    args{testAccountID, testInitiatorID, testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "create user forbidden",
			setupMocks: func(
				_ *service_mocks.AccountMock,
				_ *service_mocks.AuthMock,
				_ *service_mocks.AccountRoleMock,
				_ *service_mocks.UserMock,
				_ *service_mocks.EmailMock,
				access *service_mocks.AccessMock,
				_ *repository_mocks.AccountMock,
			) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageUsers).
					Return(service.ErrForbidden)
			},
			args:    args{testAccountID, testInitiatorID, testName, testSurname, testEmail},
			wantErr: service.ErrForbidden,
		},
		{
			name: "success",
			setupMocks: func(
				_ *service_mocks.AccountMock,
				auth *service_mocks.AuthMock,
				ar *service_mocks.AccountRoleMock,
				user *service_mocks.UserMock,
				email *service_mocks.EmailMock,
				access *service_mocks.AccessMock,
				_ *repository_mocks.AccountMock,
			) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageUsers,
					).Return(nil)

				role := domain.AccountRole{ID: uuid.New()}
				resultUser := domain.User{Email: testEmail}

				user.GetByEmailAndAccountIDMock.Expect(minimock.AnyContext, testEmail, testAccountID).
					Return(domain.User{}, repository.ErrNotFound)
				auth.GeneratePasswordMock.Expect().
					Return(testPassword, nil)
				auth.HashPasswordMock.Expect(testPassword).
					Return(testPasswordHash, nil)
				ar.GetDefaultMock.Expect(minimock.AnyContext, testAccountID).
					Return(role, nil)
				user.CreateMock.Expect(minimock.AnyContext, testName, testSurname, testEmail, testPasswordHash, role.ID).
					Return(resultUser, nil)
				email.SendCreateUserEmailMock.Expect(minimock.AnyContext, testEmail, testPassword).
					Return(nil)
			},
			args: args{testAccountID, testInitiatorID, testName, testSurname, testEmail},
			want: domain.User{Email: testEmail},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, repoServices *testutil.RepositoryMock) {
					tt.setupMocks(
						mockServices.Account,
						mockServices.Auth,
						mockServices.AccountRole,
						mockServices.User,
						mockServices.Email,
						mockServices.Access,
						repoServices.Account,
					)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewAccountService(r.Account, s)

					got, err := srv.CreateUser(
						t.Context(),
						tt.args.accountID,
						tt.args.initiatorID,
						tt.args.name,
						tt.args.surname,
						tt.args.email,
					)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

func TestService_Account_GetByID(t *testing.T) {
	t.Parallel()

	accountID := uuid.New()
	expected := []domain.Account{{ID: accountID}}

	var errSomeError = errors.New("some error")

	type args struct {
		ids []uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(*repository_mocks.AccountMock)
		args       args
		want       []domain.Account
		wantErr    error
	}{
		{
			name: "success",
			setupMocks: func(repo *repository_mocks.AccountMock) {
				repo.SelectByIDMock.Expect(minimock.AnyContext, accountID).
					Return(expected, nil)
			},
			args:    args{[]uuid.UUID{accountID}},
			want:    expected,
			wantErr: nil,
		},
		{
			name: "repo error",
			setupMocks: func(repo *repository_mocks.AccountMock) {
				repo.SelectByIDMock.Expect(minimock.AnyContext, accountID).
					Return(nil, errSomeError)
			},
			args:    args{[]uuid.UUID{accountID}},
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
					tt.setupMocks(mockRepos.Account)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewAccountService(r.Account, s)

					got, err := srv.GetByID(t.Context(), tt.args.ids...)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

func TestService_Account_GetByUserEmail(t *testing.T) {
	t.Parallel()

	email := testutil.Faker.Person().Contact().Email
	userID := uuid.New()
	roleID := uuid.New()
	accountID := uuid.New()
	var errSomeError = errors.New("some error")

	type args struct {
		email string
	}

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.UserMock,
			*service_mocks.AccountRoleMock,
			*repository_mocks.AccountMock,
		)
		args    args
		want    []domain.Account
		wantErr error
	}{
		{
			name: "user service error",
			setupMocks: func(user *service_mocks.UserMock, _ *service_mocks.AccountRoleMock, _ *repository_mocks.AccountMock) {
				user.GetByEmailMock.Expect(minimock.AnyContext, email).
					Return(nil, errSomeError)
			},
			args:    args{email},
			want:    nil,
			wantErr: errSomeError,
		},
		{
			name: "account role get error",
			setupMocks: func(user *service_mocks.UserMock, role *service_mocks.AccountRoleMock, _ *repository_mocks.AccountMock) {
				users := []domain.User{
					{ID: userID, RoleID: roleID},
				}
				user.GetByEmailMock.Expect(minimock.AnyContext, email).
					Return(users, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, roleID).
					Return(nil, errSomeError)
			},
			args:    args{email},
			want:    nil,
			wantErr: errSomeError,
		},
		{
			name: "get by id error",
			setupMocks: func(user *service_mocks.UserMock, role *service_mocks.AccountRoleMock, repo *repository_mocks.AccountMock) {
				users := []domain.User{
					{ID: userID, RoleID: roleID},
				}
				// ID роли намеренно отличается от AccountID — аккаунты возвращаются по
				// AccountID роли, а не по её собственному ID (В-18).
				accountRoles := []domain.AccountRole{
					{ID: roleID, AccountID: accountID},
				}
				user.GetByEmailMock.Expect(minimock.AnyContext, email).
					Return(users, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, roleID).
					Return(accountRoles, nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, accountID).
					Return(nil, errSomeError)
			},
			args:    args{email},
			want:    nil,
			wantErr: errSomeError,
		},
		{
			name: "success",
			setupMocks: func(user *service_mocks.UserMock, role *service_mocks.AccountRoleMock, repo *repository_mocks.AccountMock) {
				users := []domain.User{
					{ID: userID, RoleID: roleID},
				}
				// ID роли намеренно отличается от AccountID — аккаунты возвращаются по
				// AccountID роли, а не по её собственному ID (В-18).
				accountRoles := []domain.AccountRole{
					{ID: roleID, AccountID: accountID},
				}
				accounts := []domain.Account{
					{ID: accountID, Email: email},
				}
				user.GetByEmailMock.Expect(minimock.AnyContext, email).
					Return(users, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, roleID).
					Return(accountRoles, nil)
				repo.SelectByIDMock.Expect(minimock.AnyContext, accountID).
					Return(accounts, nil)
			},
			args:    args{email},
			want:    []domain.Account{{ID: accountID, Email: email}},
			wantErr: nil,
		},
		{
			name: "deactivated row excluded, empty result without account lookups",
			setupMocks: func(
				user *service_mocks.UserMock, _ *service_mocks.AccountRoleMock, _ *repository_mocks.AccountMock,
			) {
				deactivatedAt := time.Now()
				users := []domain.User{
					{ID: userID, RoleID: roleID, DeactivatedAt: &deactivatedAt},
				}
				user.GetByEmailMock.Expect(minimock.AnyContext, email).
					Return(users, nil)
			},
			args:    args{email},
			want:    []domain.Account{},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(
						mockServices.User,
						mockServices.AccountRole,
						mockRepos.Account,
					)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewAccountService(r.Account, s)

					got, err := srv.GetByUserEmail(t.Context(), tt.args.email)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}
