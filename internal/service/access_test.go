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
			args:    args{testAccountID, testInitiatorID, domain.AccountPermissionManageUsers},
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
			args:    args{testAccountID, testInitiatorID, domain.AccountPermissionManageUsers},
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
			args:    args{testAccountID, testInitiatorID, domain.AccountPermissionManageUsers},
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
			args:    args{testAccountID, testInitiatorID, domain.AccountPermissionManageUsers},
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
					Return([]domain.AccountRole{{PermissionMask: domain.SetBits(0, domain.AccountPermissionManageUsers)}}, nil)
			},
			args:    args{testAccountID, testInitiatorID, domain.AccountPermissionManageUsers},
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
			args:    args{testAccountID, testInitiatorID, domain.AccountPermissionManageUsers},
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

// TestService_Access_IsCheckGroupAction покрывает общую OR-логику доступа к операциям над
// группой (§3.1 дизайна эпика Э2, В-25): группа должна принадлежать accountID (иначе
// ErrNotFound); право уровня аккаунта (в т.ч. владелец) разрешает без проверки членства;
// иначе — членство инициатора в группе и Owner/groupAction в маске его групповой роли,
// отсутствие членства — ErrForbidden.
func TestService_Access_IsCheckGroupAction(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testGroupID := uuid.New()
	testRoleID := uuid.New()
	testGroupRoleID := uuid.New()

	errNoRows := errors.New("sql: no rows in result set")

	type args struct {
		accountID     uuid.UUID
		initiatorID   uuid.UUID
		groupID       uuid.UUID
		accountAction domain.PermissionFlag
		groupAction   domain.PermissionFlag
	}

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.UserGroupMock,
			*service_mocks.AccountMock,
			*service_mocks.UserMock,
			*service_mocks.AccountRoleMock,
			*service_mocks.GroupMemberMock,
			*service_mocks.GroupRoleMock,
		)
		args    args
		wantErr error
	}{
		{
			name: "group from another account is not found",
			setupMocks: func(
				userGroup *service_mocks.UserGroupMock,
				_ *service_mocks.AccountMock,
				_ *service_mocks.UserMock,
				_ *service_mocks.AccountRoleMock,
				_ *service_mocks.GroupMemberMock,
				_ *service_mocks.GroupRoleMock,
			) {
				userGroup.GetByIDMock.Expect(minimock.AnyContext, testGroupID).
					Return([]domain.UserGroup{{ID: testGroupID, AccountID: uuid.New()}}, nil)
			},
			args: args{
				testAccountID, testInitiatorID, testGroupID,
				domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
			},
			wantErr: service.ErrNotFound,
		},
		{
			name: "group not found propagates not found",
			setupMocks: func(
				userGroup *service_mocks.UserGroupMock,
				_ *service_mocks.AccountMock,
				_ *service_mocks.UserMock,
				_ *service_mocks.AccountRoleMock,
				_ *service_mocks.GroupMemberMock,
				_ *service_mocks.GroupRoleMock,
			) {
				userGroup.GetByIDMock.Expect(minimock.AnyContext, testGroupID).
					Return(nil, repository.ErrNotFound)
			},
			args: args{
				testAccountID, testInitiatorID, testGroupID,
				domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
			},
			wantErr: service.ErrNotFound,
		},
		{
			name: "account level action allows without group membership",
			setupMocks: func(
				userGroup *service_mocks.UserGroupMock,
				acc *service_mocks.AccountMock,
				user *service_mocks.UserMock,
				role *service_mocks.AccountRoleMock,
				_ *service_mocks.GroupMemberMock,
				_ *service_mocks.GroupRoleMock,
			) {
				userGroup.GetByIDMock.Expect(minimock.AnyContext, testGroupID).
					Return([]domain.UserGroup{{ID: testGroupID, AccountID: testAccountID}}, nil)
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{PermissionMask: domain.SetBits(0, domain.AccountPermissionManageGroups)}}, nil)
			},
			args: args{
				testAccountID, testInitiatorID, testGroupID,
				domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
			},
			wantErr: nil,
		},
		{
			name: "account owner allows without group membership",
			setupMocks: func(
				userGroup *service_mocks.UserGroupMock,
				acc *service_mocks.AccountMock,
				user *service_mocks.UserMock,
				role *service_mocks.AccountRoleMock,
				_ *service_mocks.GroupMemberMock,
				_ *service_mocks.GroupRoleMock,
			) {
				userGroup.GetByIDMock.Expect(minimock.AnyContext, testGroupID).
					Return([]domain.UserGroup{{ID: testGroupID, AccountID: testAccountID}}, nil)
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{PermissionMask: domain.SetBits(0, domain.AccountPermissionOwner)}}, nil)
			},
			args: args{
				testAccountID, testInitiatorID, testGroupID,
				domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
			},
			wantErr: nil,
		},
		{
			name: "group level action allows",
			setupMocks: func(
				userGroup *service_mocks.UserGroupMock,
				acc *service_mocks.AccountMock,
				user *service_mocks.UserMock,
				role *service_mocks.AccountRoleMock,
				groupMember *service_mocks.GroupMemberMock,
				groupRole *service_mocks.GroupRoleMock,
			) {
				userGroup.GetByIDMock.Expect(minimock.AnyContext, testGroupID).
					Return([]domain.UserGroup{{ID: testGroupID, AccountID: testAccountID}}, nil)
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{PermissionMask: domain.PermissionMask(0)}}, nil)
				groupMember.GetByUserIDAndGroupIDMock.
					Expect(minimock.AnyContext, testInitiatorID, testGroupID).
					Return(domain.GroupMember{RoleID: testGroupRoleID}, nil)
				groupRole.GetByIDMock.Expect(minimock.AnyContext, testGroupRoleID).
					Return([]domain.GroupRole{{
						ID:             testGroupRoleID,
						PermissionMask: domain.SetBits(0, domain.GroupPermissionManageMembers),
					}}, nil)
			},
			args: args{
				testAccountID, testInitiatorID, testGroupID,
				domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
			},
			wantErr: nil,
		},
		{
			name: "no membership is forbidden, not 500",
			setupMocks: func(
				userGroup *service_mocks.UserGroupMock,
				acc *service_mocks.AccountMock,
				user *service_mocks.UserMock,
				role *service_mocks.AccountRoleMock,
				groupMember *service_mocks.GroupMemberMock,
				_ *service_mocks.GroupRoleMock,
			) {
				userGroup.GetByIDMock.Expect(minimock.AnyContext, testGroupID).
					Return([]domain.UserGroup{{ID: testGroupID, AccountID: testAccountID}}, nil)
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{PermissionMask: domain.PermissionMask(0)}}, nil)
				groupMember.GetByUserIDAndGroupIDMock.
					Expect(minimock.AnyContext, testInitiatorID, testGroupID).
					Return(domain.GroupMember{}, errNoRows)
			},
			args: args{
				testAccountID, testInitiatorID, testGroupID,
				domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
			},
			wantErr: service.ErrForbidden,
		},
		{
			name: "membership without required bit is forbidden",
			setupMocks: func(
				userGroup *service_mocks.UserGroupMock,
				acc *service_mocks.AccountMock,
				user *service_mocks.UserMock,
				role *service_mocks.AccountRoleMock,
				groupMember *service_mocks.GroupMemberMock,
				groupRole *service_mocks.GroupRoleMock,
			) {
				userGroup.GetByIDMock.Expect(minimock.AnyContext, testGroupID).
					Return([]domain.UserGroup{{ID: testGroupID, AccountID: testAccountID}}, nil)
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{PermissionMask: domain.PermissionMask(0)}}, nil)
				groupMember.GetByUserIDAndGroupIDMock.
					Expect(minimock.AnyContext, testInitiatorID, testGroupID).
					Return(domain.GroupMember{RoleID: testGroupRoleID}, nil)
				groupRole.GetByIDMock.Expect(minimock.AnyContext, testGroupRoleID).
					Return([]domain.GroupRole{{ID: testGroupRoleID, PermissionMask: domain.PermissionMask(0)}}, nil)
			},
			args: args{
				testAccountID, testInitiatorID, testGroupID,
				domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
			},
			wantErr: service.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, _ *testutil.RepositoryMock) {
					tt.setupMocks(
						mockServices.UserGroup,
						mockServices.Account,
						mockServices.User,
						mockServices.AccountRole,
						mockServices.GroupMember,
						mockServices.GroupRole,
					)
				},
				func(s *service.Service, _ *repository.Repository) {
					srv := service.NewAccessService(s)

					err := srv.IsCheckGroupAction(
						t.Context(),
						tt.args.accountID, tt.args.initiatorID, tt.args.groupID,
						tt.args.accountAction, tt.args.groupAction,
					)

					if tt.wantErr == nil {
						require.NoError(t, err)
					} else {
						require.ErrorIs(t, err, tt.wantErr)
					}
				},
			)
		})
	}
}
