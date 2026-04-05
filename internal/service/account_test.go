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

func TestService_Account_Create(t *testing.T) {
	t.Parallel()

	var (
		testName         = testutil.Faker.Person().FirstName()
		testSurname      = testutil.Faker.Person().LastName()
		testEmail        = testutil.Faker.Person().Contact().Email
		testInvalid      = "invalid"
		testPassword     = testutil.Faker.Person().Name()
		testPasswordHash = testutil.Faker.Hash().MD5()
	)

	testAccountName, _ := domain.NameFromEmail(testEmail)

	successAccount := domain.Account{ID: uuid.New()}

	var errSomeError = errors.New("some error")

	type args struct {
		name    string
		surname string
		email   string
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
			name: "invalid email",
			setupMocks: func(t *testing.T,
				ar *service_mocks.AccountRoleMock,
				auth *service_mocks.AuthMock,
				user *service_mocks.UserMock,
				email *service_mocks.EmailMock,
				repo *repository_mocks.AccountMock,
			) {
			},
			args:    args{testName, testSurname, testInvalid},
			want:    domain.Account{},
			wantErr: service.ErrEmailInvalid,
		},
		{
			name: "duplicate account name",
			setupMocks: func(t *testing.T,
				ar *service_mocks.AccountRoleMock,
				auth *service_mocks.AuthMock,
				user *service_mocks.UserMock,
				email *service_mocks.EmailMock,
				repo *repository_mocks.AccountMock,
			) {
				repo.InsertMock.Expect(minimock.AnyContext, testAccountName, testEmail).
					Return(domain.Account{}, dberrors.AccountErrors.ErrUniqueAccountsNameKey)
			},
			args:    args{testName, testSurname, testEmail},
			wantErr: service.ErrAccountNameExists,
		},
		{
			name: "insert error",
			setupMocks: func(t *testing.T,
				ar *service_mocks.AccountRoleMock,
				auth *service_mocks.AuthMock,
				user *service_mocks.UserMock,
				email *service_mocks.EmailMock,
				repo *repository_mocks.AccountMock,
			) {
				repo.InsertMock.Expect(minimock.AnyContext, testAccountName, testEmail).
					Return(domain.Account{}, errSomeError)
			},
			args:    args{testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "owner role error",
			setupMocks: func(t *testing.T,
				ar *service_mocks.AccountRoleMock,
				auth *service_mocks.AuthMock,
				user *service_mocks.UserMock,
				email *service_mocks.EmailMock,
				repo *repository_mocks.AccountMock,
			) {
				acc := domain.Account{ID: uuid.New()}

				repo.InsertMock.Expect(minimock.AnyContext, testAccountName, testEmail).
					Return(acc, nil)

				ar.CreateSystemAccountOwnerMock.Expect(minimock.AnyContext, acc.ID).
					Return(domain.AccountRole{}, errSomeError)
			},
			args:    args{testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "generate password error",
			setupMocks: func(t *testing.T,
				ar *service_mocks.AccountRoleMock,
				auth *service_mocks.AuthMock,
				user *service_mocks.UserMock,
				email *service_mocks.EmailMock,
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
			args:    args{testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "hash password error",
			setupMocks: func(t *testing.T,
				ar *service_mocks.AccountRoleMock,
				auth *service_mocks.AuthMock,
				user *service_mocks.UserMock,
				email *service_mocks.EmailMock,
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
			args:    args{testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "user create error",
			setupMocks: func(t *testing.T,
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
					Return(domain.User{}, errSomeError)
			},
			args:    args{testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "email send error",
			setupMocks: func(t *testing.T,
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
			args:    args{testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "success",
			setupMocks: func(t *testing.T,
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
			args:    args{testName, testSurname, testEmail},
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

					got, err := srv.Create(t.Context(), tt.args.name, tt.args.surname, tt.args.email)

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
		testName    = testutil.Faker.Person().FirstName()
		testSurname = testutil.Faker.Person().LastName()
		testEmail   = testutil.Faker.Person().Contact().Email
		accountID   = uuid.New()
		password    = testutil.Faker.Person().Name()
	)

	var errSomeError = errors.New("some error")

	type args struct {
		accountID uuid.UUID
		name      string
		surname   string
		email     string
	}

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.AccountMock,
			*service_mocks.AuthMock,
			*service_mocks.AccountRoleMock,
			*service_mocks.UserMock,
			*service_mocks.EmailMock,
		)
		args    args
		want    domain.User
		wantErr error
	}{
		{
			name: "user already exists",
			setupMocks: func(
				acc *service_mocks.AccountMock,
				auth *service_mocks.AuthMock,
				ar *service_mocks.AccountRoleMock,
				user *service_mocks.UserMock,
				email *service_mocks.EmailMock,
			) {
				acc.IsExistsUserByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return(true, nil)
			},
			args:    args{accountID, testName, testSurname, testEmail},
			wantErr: service.ErrAccountUserExists,
		},
		{
			name: "generate password error",
			setupMocks: func(
				acc *service_mocks.AccountMock,
				auth *service_mocks.AuthMock,
				ar *service_mocks.AccountRoleMock,
				user *service_mocks.UserMock,
				email *service_mocks.EmailMock,
			) {
				acc.IsExistsUserByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return(false, nil)

				auth.GeneratePasswordMock.Expect().
					Return("", errSomeError)
			},
			args:    args{accountID, testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "get default role error",
			setupMocks: func(
				acc *service_mocks.AccountMock,
				auth *service_mocks.AuthMock,
				ar *service_mocks.AccountRoleMock,
				user *service_mocks.UserMock,
				email *service_mocks.EmailMock,
			) {
				acc.IsExistsUserByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return(false, nil)

				auth.GeneratePasswordMock.Expect().
					Return(password, nil)

				ar.GetDefaultMock.Expect(minimock.AnyContext, accountID).
					Return(domain.AccountRole{}, errSomeError)
			},
			args:    args{accountID, testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "user create error",
			setupMocks: func(
				acc *service_mocks.AccountMock,
				auth *service_mocks.AuthMock,
				ar *service_mocks.AccountRoleMock,
				user *service_mocks.UserMock,
				email *service_mocks.EmailMock,
			) {
				role := domain.AccountRole{ID: uuid.New()}

				acc.IsExistsUserByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return(false, nil)

				auth.GeneratePasswordMock.Expect().
					Return(password, nil)

				ar.GetDefaultMock.Expect(minimock.AnyContext, accountID).
					Return(role, nil)

				user.CreateMock.Expect(minimock.AnyContext, testName, testSurname, testEmail, password, role.ID).
					Return(domain.User{}, errSomeError)
			},
			args:    args{accountID, testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "email send error",
			setupMocks: func(
				acc *service_mocks.AccountMock,
				auth *service_mocks.AuthMock,
				ar *service_mocks.AccountRoleMock,
				user *service_mocks.UserMock,
				email *service_mocks.EmailMock,
			) {
				role := domain.AccountRole{ID: uuid.New()}

				acc.IsExistsUserByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return(false, nil)

				auth.GeneratePasswordMock.Expect().
					Return(password, nil)

				ar.GetDefaultMock.Expect(minimock.AnyContext, accountID).
					Return(role, nil)

				user.CreateMock.Expect(minimock.AnyContext, testName, testSurname, testEmail, password, role.ID).
					Return(domain.User{Email: testEmail}, nil)

				email.SendCreateUserEmailMock.Expect(minimock.AnyContext, testEmail, password).
					Return(errSomeError)
			},
			args:    args{accountID, testName, testSurname, testEmail},
			wantErr: errSomeError,
		},
		{
			name: "success",
			setupMocks: func(
				acc *service_mocks.AccountMock,
				auth *service_mocks.AuthMock,
				ar *service_mocks.AccountRoleMock,
				user *service_mocks.UserMock,
				email *service_mocks.EmailMock,
			) {
				role := domain.AccountRole{ID: uuid.New()}
				resultUser := domain.User{Email: testEmail}

				acc.IsExistsUserByEmailMock.Expect(minimock.AnyContext, testEmail).
					Return(false, nil)

				auth.GeneratePasswordMock.Expect().
					Return(password, nil)

				ar.GetDefaultMock.Expect(minimock.AnyContext, accountID).
					Return(role, nil)

				user.CreateMock.Expect(minimock.AnyContext, testName, testSurname, testEmail, password, role.ID).
					Return(resultUser, nil)

				email.SendCreateUserEmailMock.Expect(minimock.AnyContext, testEmail, password).
					Return(nil)
			},
			args: args{accountID, testName, testSurname, testEmail},
			want: domain.User{Email: testEmail},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, _ *testutil.RepositoryMock) {
					tt.setupMocks(
						mockServices.Account,
						mockServices.Auth,
						mockServices.AccountRole,
						mockServices.User,
						mockServices.Email,
					)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewAccountService(r.Account, s)

					got, err := srv.CreateUser(
						t.Context(),
						tt.args.accountID,
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

func TestService_Account_IsExistsUserByEmail(t *testing.T) {
	t.Parallel()

	email := testutil.Faker.Person().Contact().Email

	var errSomeError = errors.New("some error")

	type args struct {
		email string
	}

	tests := []struct {
		name       string
		setupMocks func(*service_mocks.AccountMock)
		args       args
		want       bool
		wantErr    error
	}{
		{
			name: "exists",
			setupMocks: func(acc *service_mocks.AccountMock) {
				acc.GetByUserEmailMock.Expect(minimock.AnyContext, email).
					Return([]domain.Account{{Email: email}}, nil)
			},
			args:    args{email},
			want:    true,
			wantErr: nil,
		},
		{
			name: "not exists",
			setupMocks: func(acc *service_mocks.AccountMock) {
				acc.GetByUserEmailMock.Expect(minimock.AnyContext, email).
					Return([]domain.Account{{Email: "other@mail.com"}}, nil)
			},
			args:    args{email},
			want:    false,
			wantErr: nil,
		},
		{
			name: "service error",
			setupMocks: func(acc *service_mocks.AccountMock) {
				acc.GetByUserEmailMock.Expect(minimock.AnyContext, email).
					Return(nil, errSomeError)
			},
			args:    args{email},
			want:    false,
			wantErr: errSomeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, _ *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Account)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewAccountService(r.Account, s)

					got, err := srv.IsExistsUserByEmail(t.Context(), tt.args.email)

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
			setupMocks: func(user *service_mocks.UserMock, role *service_mocks.AccountRoleMock, repo *repository_mocks.AccountMock) {
				user.GetByEmailMock.Expect(minimock.AnyContext, email).
					Return(nil, errSomeError)
			},
			args:    args{email},
			want:    nil,
			wantErr: errSomeError,
		},
		{
			name: "account role get error",
			setupMocks: func(user *service_mocks.UserMock, role *service_mocks.AccountRoleMock, repo *repository_mocks.AccountMock) {
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
				accountRoles := []domain.AccountRole{
					{ID: accountID},
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
				accountRoles := []domain.AccountRole{
					{ID: accountID},
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
