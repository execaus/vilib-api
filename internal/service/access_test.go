package service_test

import (
	"errors"
	"testing"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
	"vilib-api/internal/service"
	"vilib-api/internal/service/service_mocks"
	"vilib-api/testutil"

	"github.com/gojuno/minimock/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestService_Access_IsCheckAccountAction(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testRoleID := uuid.New()

	var errSomeError = errors.New("some error")

	type args struct {
		accountID   uuid.UUID
		initiatorID uuid.UUID
		action      domain.PermissionFlag
	}

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.AccountMock,
			*service_mocks.UserMock,
			*service_mocks.AccountRoleMock,
		)
		args    args
		wantErr error
	}{
		{
			name: "user not in account",
			setupMocks: func(acc *service_mocks.AccountMock, user *service_mocks.UserMock, role *service_mocks.AccountRoleMock) {
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(service.ErrForbidden)
			},
			args:    args{testAccountID, testInitiatorID, domain.AccountPermissionCreateUser},
			wantErr: service.ErrForbidden,
		},
		{
			name: "get user error",
			setupMocks: func(acc *service_mocks.AccountMock, user *service_mocks.UserMock, role *service_mocks.AccountRoleMock) {
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return(nil, errSomeError)
			},
			args:    args{testAccountID, testInitiatorID, domain.AccountPermissionCreateUser},
			wantErr: errSomeError,
		},
		{
			name: "get role error",
			setupMocks: func(acc *service_mocks.AccountMock, user *service_mocks.UserMock, role *service_mocks.AccountRoleMock) {
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return(nil, errSomeError)
			},
			args:    args{testAccountID, testInitiatorID, domain.AccountPermissionCreateUser},
			wantErr: errSomeError,
		},
		{
			name: "owner has access",
			setupMocks: func(acc *service_mocks.AccountMock, user *service_mocks.UserMock, role *service_mocks.AccountRoleMock) {
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{PermissionMask: domain.SetBits(0, domain.AccountPermissionOwner)}}, nil)
			},
			args:    args{testAccountID, testInitiatorID, domain.AccountPermissionCreateUser},
			wantErr: nil,
		},
		{
			name: "has permission",
			setupMocks: func(acc *service_mocks.AccountMock, user *service_mocks.UserMock, role *service_mocks.AccountRoleMock) {
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{PermissionMask: domain.SetBits(0, domain.AccountPermissionCreateUser)}}, nil)
			},
			args:    args{testAccountID, testInitiatorID, domain.AccountPermissionCreateUser},
			wantErr: nil,
		},
		{
			name: "no permission",
			setupMocks: func(acc *service_mocks.AccountMock, user *service_mocks.UserMock, role *service_mocks.AccountRoleMock) {
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{PermissionMask: domain.PermissionMask(0)}}, nil)
			},
			args:    args{testAccountID, testInitiatorID, domain.AccountPermissionCreateUser},
			wantErr: service.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, _ *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Account, mockServices.User, mockServices.AccountRole)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewAccessService(s)

					err := srv.IsCheckAccountAction(t.Context(), tt.args.accountID, tt.args.initiatorID, tt.args.action)

					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}
