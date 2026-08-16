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

func TestService_GroupMember_Create(t *testing.T) {
	t.Parallel()

	testGroupID := uuid.New()
	testRoleID := uuid.New()
	testUserID := uuid.New()

	var errSomeError = errors.New("some error")

	type args struct {
		groupID uuid.UUID
		roleID  uuid.UUID
		usersID []uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(*repository_mocks.GroupMemberMock)
		args       args
		want       []domain.GroupMember
		wantErr    error
	}{
		{
			name: "success",
			setupMocks: func(repo *repository_mocks.GroupMemberMock) {
				repo.InsertMock.Expect(minimock.AnyContext, testGroupID, testRoleID, testUserID).
					Return([]domain.GroupMember{{GroupID: testGroupID, UserID: testUserID}}, nil)
			},
			args:    args{testGroupID, testRoleID, []uuid.UUID{testUserID}},
			want:    []domain.GroupMember{{GroupID: testGroupID, UserID: testUserID}},
			wantErr: nil,
		},
		{
			name: "insert error",
			setupMocks: func(repo *repository_mocks.GroupMemberMock) {
				repo.InsertMock.Expect(minimock.AnyContext, testGroupID, testRoleID, testUserID).
					Return(nil, errSomeError)
			},
			args:    args{testGroupID, testRoleID, []uuid.UUID{testUserID}},
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
					tt.setupMocks(mockRepos.GroupMember)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewGroupMemberService(r.GroupMember, s)

					got, err := srv.Create(t.Context(), tt.args.groupID, tt.args.roleID, tt.args.usersID...)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

// TestService_GroupMember_RemoveMember покрывает OR-логику удаления участника (В-18, §6.4 ТЗ):
// инициатор с аккаунтным правом ManageGroups (в т.ч. владелец аккаунта) удаляет участника из
// любой группы своего аккаунта без членства в ней; иначе действует групповая проверка
// (Owner/ManageMembers в самой группе), а отсутствие членства даёт 403, а не 500.
func TestService_GroupMember_RemoveMember(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testGroupID := uuid.New()
	testTargetID := uuid.New()
	testInitiatorGroupRoleID := uuid.New()

	errNoRows := errors.New("sql: no rows in result set")

	type args struct {
		accountID   uuid.UUID
		initiatorID uuid.UUID
		groupID     uuid.UUID
		targetID    uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.AccessMock,
			*service_mocks.UserGroupMock,
			*service_mocks.GroupRoleMock,
			*repository_mocks.GroupMemberMock,
		)
		args    args
		wantErr error
	}{
		{
			name: "account owner removes member without group membership",
			setupMocks: func(
				access *service_mocks.AccessMock,
				userGroup *service_mocks.UserGroupMock,
				groupRole *service_mocks.GroupRoleMock,
				repo *repository_mocks.GroupMemberMock,
			) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(nil)
				userGroup.GetAllMock.Expect(minimock.AnyContext, testInitiatorID, testAccountID).
					Return([]domain.UserGroup{{ID: testGroupID, AccountID: testAccountID}}, nil)
				repo.DeleteMock.Expect(minimock.AnyContext, testGroupID, testTargetID).
					Return(nil)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID, testTargetID},
			wantErr: nil,
		},
		{
			name: "account owner but group belongs to another account is forbidden",
			setupMocks: func(
				access *service_mocks.AccessMock,
				userGroup *service_mocks.UserGroupMock,
				groupRole *service_mocks.GroupRoleMock,
				repo *repository_mocks.GroupMemberMock,
			) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(nil)
				userGroup.GetAllMock.Expect(minimock.AnyContext, testInitiatorID, testAccountID).
					Return([]domain.UserGroup{{ID: uuid.New(), AccountID: testAccountID}}, nil)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID, testTargetID},
			wantErr: service.ErrForbidden,
		},
		{
			name: "user without account permission and without group membership is forbidden, not 500",
			setupMocks: func(
				access *service_mocks.AccessMock,
				userGroup *service_mocks.UserGroupMock,
				groupRole *service_mocks.GroupRoleMock,
				repo *repository_mocks.GroupMemberMock,
			) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(service.ErrForbidden)
				repo.SelectByUserIDAndGroupIDMock.
					Expect(minimock.AnyContext, testInitiatorID, testGroupID).
					Return(domain.GroupMember{}, errNoRows)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID, testTargetID},
			wantErr: service.ErrForbidden,
		},
		{
			name: "group member with ManageMembers removes member",
			setupMocks: func(
				access *service_mocks.AccessMock,
				userGroup *service_mocks.UserGroupMock,
				groupRole *service_mocks.GroupRoleMock,
				repo *repository_mocks.GroupMemberMock,
			) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(service.ErrForbidden)
				repo.SelectByUserIDAndGroupIDMock.
					Expect(minimock.AnyContext, testInitiatorID, testGroupID).
					Return(domain.GroupMember{RoleID: testInitiatorGroupRoleID}, nil)
				groupRole.GetByIDMock.Expect(minimock.AnyContext, testInitiatorGroupRoleID).
					Return([]domain.GroupRole{{
						ID:             testInitiatorGroupRoleID,
						PermissionMask: domain.SetBits(0, domain.GroupPermissionManageMembers),
					}}, nil)
				repo.DeleteMock.Expect(minimock.AnyContext, testGroupID, testTargetID).
					Return(nil)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID, testTargetID},
			wantErr: nil,
		},
		{
			name: "group member without rights is forbidden",
			setupMocks: func(
				access *service_mocks.AccessMock,
				userGroup *service_mocks.UserGroupMock,
				groupRole *service_mocks.GroupRoleMock,
				repo *repository_mocks.GroupMemberMock,
			) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(service.ErrForbidden)
				repo.SelectByUserIDAndGroupIDMock.
					Expect(minimock.AnyContext, testInitiatorID, testGroupID).
					Return(domain.GroupMember{RoleID: testInitiatorGroupRoleID}, nil)
				groupRole.GetByIDMock.Expect(minimock.AnyContext, testInitiatorGroupRoleID).
					Return([]domain.GroupRole{{ID: testInitiatorGroupRoleID, PermissionMask: 0}}, nil)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID, testTargetID},
			wantErr: service.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(
						mockServices.Access,
						mockServices.UserGroup,
						mockServices.GroupRole,
						mockRepos.GroupMember,
					)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewGroupMemberService(r.GroupMember, s)

					err := srv.RemoveMember(t.Context(), tt.args.accountID, tt.args.initiatorID, tt.args.groupID, tt.args.targetID)

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
