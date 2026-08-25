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

// TestService_Access_CanManageAssignments проверяет право на управление назначениями (§2
// дизайна эпика Э3, решение В-3): тонкая обёртка над IsCheckGroupAction с фиксированными
// битами ManageAssignments — детальные ветки OR-логики покрыты
// TestService_Access_IsCheckGroupAction.
func TestService_Access_CanManageAssignments(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testGroupID := uuid.New()
	testRoleID := uuid.New()
	testGroupRoleID := uuid.New()

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
		wantErr error
	}{
		{
			name: "account permission manage assignments grants access",
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
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{
						{PermissionMask: domain.SetBits(0, domain.AccountPermissionManageAssignments)},
					}, nil)
			},
			wantErr: nil,
		},
		{
			name: "group permission manage assignments grants access",
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
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).Return(nil)
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
						PermissionMask: domain.SetBits(0, domain.GroupPermissionManageAssignments),
					}}, nil)
			},
			wantErr: nil,
		},
		{
			name: "no permission is forbidden",
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
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{PermissionMask: domain.PermissionMask(0)}}, nil)
				groupMember.GetByUserIDAndGroupIDMock.
					Expect(minimock.AnyContext, testInitiatorID, testGroupID).
					Return(domain.GroupMember{}, errors.New("not a member"))
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

					err := srv.CanManageAssignments(t.Context(), testAccountID, testInitiatorID, testGroupID)

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

// TestService_Access_CanWatchVideo проверяет OR-логику допуска к просмотру видео произвольного
// пользователя (§0 дизайна эпика Э3 — вынесенная логика бывшего VideoService.canWatch):
// аккаунтное VideoWatch/ManageVideo либо, при членстве, групповое VideoWatch/ManageVideo/Owner.
func TestService_Access_CanWatchVideo(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testUserID := uuid.New()
	testGroupID := uuid.New()
	testRoleID := uuid.New()
	testGroupRoleID := uuid.New()

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.AccountMock,
			*service_mocks.UserMock,
			*service_mocks.AccountRoleMock,
			*service_mocks.GroupMemberMock,
			*service_mocks.GroupRoleMock,
		)
		want bool
	}{
		{
			name: "account video watch grants access",
			setupMocks: func(
				acc *service_mocks.AccountMock,
				user *service_mocks.UserMock,
				role *service_mocks.AccountRoleMock,
				_ *service_mocks.GroupMemberMock,
				_ *service_mocks.GroupRoleMock,
			) {
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testUserID).Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{PermissionMask: domain.SetBits(0, domain.AccountPermissionVideoWatch)}}, nil)
			},
			want: true,
		},
		{
			name: "account manage video grants access without video watch",
			setupMocks: func(
				acc *service_mocks.AccountMock,
				user *service_mocks.UserMock,
				role *service_mocks.AccountRoleMock,
				_ *service_mocks.GroupMemberMock,
				_ *service_mocks.GroupRoleMock,
			) {
				acc.IsHasUserMock.When(minimock.AnyContext, testAccountID, testUserID).Then(nil)
				user.GetByIDMock.When(minimock.AnyContext, testUserID).
					Then([]domain.User{{ID: testUserID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.When(minimock.AnyContext, testRoleID).
					Then([]domain.AccountRole{{PermissionMask: domain.SetBits(0, domain.AccountPermissionManageVideo)}}, nil)
			},
			want: true,
		},
		{
			name: "group video watch grants access without account rights",
			setupMocks: func(
				acc *service_mocks.AccountMock,
				user *service_mocks.UserMock,
				role *service_mocks.AccountRoleMock,
				groupMember *service_mocks.GroupMemberMock,
				groupRole *service_mocks.GroupRoleMock,
			) {
				acc.IsHasUserMock.When(minimock.AnyContext, testAccountID, testUserID).Then(nil)
				user.GetByIDMock.When(minimock.AnyContext, testUserID).
					Then([]domain.User{{ID: testUserID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.When(minimock.AnyContext, testRoleID).
					Then([]domain.AccountRole{{PermissionMask: domain.PermissionMask(0)}}, nil)
				groupMember.GetByUserIDAndGroupIDMock.
					Expect(minimock.AnyContext, testUserID, testGroupID).
					Return(domain.GroupMember{RoleID: testGroupRoleID}, nil)
				groupRole.GetByIDMock.Expect(minimock.AnyContext, testGroupRoleID).
					Return([]domain.GroupRole{{
						ID:             testGroupRoleID,
						PermissionMask: domain.SetBits(0, domain.GroupPermissionVideoWatch),
					}}, nil)
			},
			want: true,
		},
		{
			name: "group owner grants access via video watch branch",
			setupMocks: func(
				acc *service_mocks.AccountMock,
				user *service_mocks.UserMock,
				role *service_mocks.AccountRoleMock,
				groupMember *service_mocks.GroupMemberMock,
				groupRole *service_mocks.GroupRoleMock,
			) {
				acc.IsHasUserMock.When(minimock.AnyContext, testAccountID, testUserID).Then(nil)
				user.GetByIDMock.When(minimock.AnyContext, testUserID).
					Then([]domain.User{{ID: testUserID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.When(minimock.AnyContext, testRoleID).
					Then([]domain.AccountRole{{PermissionMask: domain.PermissionMask(0)}}, nil)
				groupMember.GetByUserIDAndGroupIDMock.
					Expect(minimock.AnyContext, testUserID, testGroupID).
					Return(domain.GroupMember{RoleID: testGroupRoleID}, nil)
				groupRole.GetByIDMock.
					Expect(minimock.AnyContext, testGroupRoleID).
					Return([]domain.GroupRole{{
						ID:             testGroupRoleID,
						PermissionMask: domain.SetBits(0, domain.GroupPermissionOwner),
					}}, nil)
			},
			want: true,
		},
		{
			name: "no permission and no membership denies access",
			setupMocks: func(
				acc *service_mocks.AccountMock,
				user *service_mocks.UserMock,
				role *service_mocks.AccountRoleMock,
				groupMember *service_mocks.GroupMemberMock,
				_ *service_mocks.GroupRoleMock,
			) {
				acc.IsHasUserMock.When(minimock.AnyContext, testAccountID, testUserID).Then(nil)
				user.GetByIDMock.When(minimock.AnyContext, testUserID).
					Then([]domain.User{{ID: testUserID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.When(minimock.AnyContext, testRoleID).
					Then([]domain.AccountRole{{PermissionMask: domain.PermissionMask(0)}}, nil)
				groupMember.GetByUserIDAndGroupIDMock.
					When(minimock.AnyContext, testUserID, testGroupID).
					Then(domain.GroupMember{}, errors.New("not a member"))
			},
			want: false,
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
						mockServices.User,
						mockServices.AccountRole,
						mockServices.GroupMember,
						mockServices.GroupRole,
					)
				},
				func(s *service.Service, _ *repository.Repository) {
					srv := service.NewAccessService(s)

					got := srv.CanWatchVideo(t.Context(), testAccountID, testUserID, testGroupID)

					require.Equal(t, tt.want, got)
				},
			)
		})
	}
}

// TestService_Access_CanManageVideo проверяет OR-логику допуска к управлению видео группы
// (§2 дизайна эпика Э4 — вынесенная логика бывшего VideoService.canManageVideo): аккаунтное
// или групповое ManageVideo (в т.ч. Owner), иначе — отказ.
func TestService_Access_CanManageVideo(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testGroupID := uuid.New()
	testRoleID := uuid.New()
	testGroupRoleID := uuid.New()

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
		wantErr error
	}{
		{
			name: "account permission manage video grants access",
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
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{
						{PermissionMask: domain.SetBits(0, domain.AccountPermissionManageVideo)},
					}, nil)
			},
			wantErr: nil,
		},
		{
			name: "account owner grants access without explicit manage video bit",
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
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{
						{PermissionMask: domain.SetBits(0, domain.AccountPermissionOwner)},
					}, nil)
			},
			wantErr: nil,
		},
		{
			name: "group permission manage video grants access",
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
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).Return(nil)
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
						PermissionMask: domain.SetBits(0, domain.GroupPermissionManageVideo),
					}}, nil)
			},
			wantErr: nil,
		},
		{
			name: "group owner grants access without explicit manage video bit",
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
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).Return(nil)
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
						PermissionMask: domain.SetBits(0, domain.GroupPermissionOwner),
					}}, nil)
			},
			wantErr: nil,
		},
		{
			name: "no permission and no membership is forbidden",
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
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{PermissionMask: domain.PermissionMask(0)}}, nil)
				groupMember.GetByUserIDAndGroupIDMock.
					Expect(minimock.AnyContext, testInitiatorID, testGroupID).
					Return(domain.GroupMember{}, errors.New("not a member"))
			},
			wantErr: service.ErrForbidden,
		},
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
			wantErr: service.ErrNotFound,
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

					err := srv.CanManageVideo(t.Context(), testAccountID, testGroupID, testInitiatorID)

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

// TestService_Access_ManagedAssignmentGroups проверяет область чтения отчётов по назначениям
// (В-8 решение владельца): аккаунтное право ManageAssignments/Owner даёt all=true; иначе —
// список групп, где у инициатора Owner/ManageAssignments в групповой роли.
func TestService_Access_ManagedAssignmentGroups(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testRoleID := uuid.New()
	testGroupIDManaged := uuid.New()
	testGroupIDOther := uuid.New()
	testGroupRoleIDManaged := uuid.New()
	testGroupRoleIDOther := uuid.New()

	errSomeError := errors.New("some error")

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.AccountMock,
			*service_mocks.UserMock,
			*service_mocks.AccountRoleMock,
			*service_mocks.GroupMemberMock,
			*service_mocks.GroupRoleMock,
		)
		wantAll    bool
		wantGroups []uuid.UUID
		wantErr    error
	}{
		{
			name: "account permission grants all",
			setupMocks: func(
				acc *service_mocks.AccountMock,
				user *service_mocks.UserMock,
				role *service_mocks.AccountRoleMock,
				_ *service_mocks.GroupMemberMock,
				_ *service_mocks.GroupRoleMock,
			) {
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{
						{PermissionMask: domain.SetBits(0, domain.AccountPermissionManageAssignments)},
					}, nil)
			},
			wantAll: true,
		},
		{
			name: "no memberships returns empty scope",
			setupMocks: func(
				acc *service_mocks.AccountMock,
				user *service_mocks.UserMock,
				role *service_mocks.AccountRoleMock,
				groupMember *service_mocks.GroupMemberMock,
				_ *service_mocks.GroupRoleMock,
			) {
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{PermissionMask: domain.PermissionMask(0)}}, nil)
				groupMember.GetByUserIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return(nil, nil)
			},
			wantAll:    false,
			wantGroups: nil,
		},
		{
			name: "only groups with owner or manage assignments bit are included",
			setupMocks: func(
				acc *service_mocks.AccountMock,
				user *service_mocks.UserMock,
				role *service_mocks.AccountRoleMock,
				groupMember *service_mocks.GroupMemberMock,
				groupRole *service_mocks.GroupRoleMock,
			) {
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{PermissionMask: domain.PermissionMask(0)}}, nil)
				groupMember.GetByUserIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.GroupMember{
						{GroupID: testGroupIDManaged, RoleID: testGroupRoleIDManaged},
						{GroupID: testGroupIDOther, RoleID: testGroupRoleIDOther},
					}, nil)
				groupRole.GetByAccountIDMock.Expect(minimock.AnyContext, testAccountID).
					Return([]domain.GroupRole{
						{
							ID:             testGroupRoleIDManaged,
							PermissionMask: domain.SetBits(0, domain.GroupPermissionManageAssignments),
						},
						{ID: testGroupRoleIDOther, PermissionMask: domain.PermissionMask(0)},
					}, nil)
			},
			wantAll:    false,
			wantGroups: []uuid.UUID{testGroupIDManaged},
		},
		{
			name: "group member lookup error propagates",
			setupMocks: func(
				acc *service_mocks.AccountMock,
				user *service_mocks.UserMock,
				role *service_mocks.AccountRoleMock,
				groupMember *service_mocks.GroupMemberMock,
				_ *service_mocks.GroupRoleMock,
			) {
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return([]domain.User{{ID: testInitiatorID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{PermissionMask: domain.PermissionMask(0)}}, nil)
				groupMember.GetByUserIDMock.Expect(minimock.AnyContext, testInitiatorID).
					Return(nil, errSomeError)
			},
			wantErr: errSomeError,
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
						mockServices.User,
						mockServices.AccountRole,
						mockServices.GroupMember,
						mockServices.GroupRole,
					)
				},
				func(s *service.Service, _ *repository.Repository) {
					srv := service.NewAccessService(s)

					all, groups, err := srv.ManagedAssignmentGroups(t.Context(), testAccountID, testInitiatorID)

					if tt.wantErr == nil {
						require.NoError(t, err)
					} else {
						require.ErrorIs(t, err, tt.wantErr)
					}
					require.Equal(t, tt.wantAll, all)
					require.Equal(t, tt.wantGroups, groups)
				},
			)
		})
	}
}
