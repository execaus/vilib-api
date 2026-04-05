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

func TestService_AccountCreate_Success(t *testing.T) {
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
					Return(domain.Account{}, errors.New("db error"))
			},
			args:    args{testName, testSurname, testEmail},
			wantErr: errors.New("db error"),
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
					Return(domain.AccountRole{}, errors.New("role error"))
			},
			args:    args{testName, testSurname, testEmail},
			wantErr: errors.New("role error"),
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
					Return("", errors.New("gen error"))
			},
			args:    args{testName, testSurname, testEmail},
			wantErr: errors.New("gen error"),
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
					Return("", errors.New("hash error"))
			},
			args:    args{testName, testSurname, testEmail},
			wantErr: errors.New("hash error"),
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
					Return(domain.User{}, errors.New("user error"))
			},
			args:    args{testName, testSurname, testEmail},
			wantErr: errors.New("user error"),
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
					Return(errors.New("mail error"))
			},
			args:    args{testName, testSurname, testEmail},
			wantErr: errors.New("mail error"),
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
