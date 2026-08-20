package service_test

import (
	"errors"
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

// TestService_GroupMember_RemoveMember покрывает право на удаление участника через общий
// примитив Access.IsCheckGroupAction (§3.1 дизайна эпика Э2, В-25).
func TestService_GroupMember_RemoveMember(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testGroupID := uuid.New()
	testTargetID := uuid.New()

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
			*repository_mocks.GroupMemberMock,
			*service_mocks.AssignmentMock,
		)
		args    args
		wantErr error
	}{
		{
			name: "access granted removes member",
			setupMocks: func(
				access *service_mocks.AccessMock,
				repo *repository_mocks.GroupMemberMock,
				assignment *service_mocks.AssignmentMock,
			) {
				access.IsCheckGroupActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID, testInitiatorID, testGroupID,
						domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
					).Return(nil)
				repo.DeleteMock.Expect(minimock.AnyContext, testGroupID, testTargetID).
					Return(nil)
				// Каскад обязательного обучения: участия через эту группу отменяются (Э3-Т30).
				assignment.OnMemberRemovedMock.
					Expect(minimock.AnyContext, testGroupID, testTargetID).
					Return(nil)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID, testTargetID},
			wantErr: nil,
		},
		{
			name: "no access is forbidden",
			setupMocks: func(
				access *service_mocks.AccessMock,
				repo *repository_mocks.GroupMemberMock,
				assignment *service_mocks.AssignmentMock,
			) {
				access.IsCheckGroupActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID, testInitiatorID, testGroupID,
						domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
					).Return(service.ErrForbidden)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID, testTargetID},
			wantErr: service.ErrForbidden,
		},
		{
			name: "group belongs to another account is not found",
			setupMocks: func(
				access *service_mocks.AccessMock,
				repo *repository_mocks.GroupMemberMock,
				assignment *service_mocks.AssignmentMock,
			) {
				access.IsCheckGroupActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID, testInitiatorID, testGroupID,
						domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
					).Return(service.ErrNotFound)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID, testTargetID},
			wantErr: service.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Access, mockRepos.GroupMember, mockServices.Assignment)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewGroupMemberService(r.GroupMember, s)

					err := srv.RemoveMember(
						t.Context(), tt.args.accountID, tt.args.initiatorID, tt.args.groupID, tt.args.targetID,
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

// TestService_GroupMember_ListByGroup покрывает список участников группы (§3.2 дизайна эпика
// Э2, П-3): право — Access.IsCheckGroupAction(ManageGroups, GroupPermissionManageMembers);
// членства → батч пользователей → роли группы аккаунта → сборка карточек.
func TestService_GroupMember_ListByGroup(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testGroupID := uuid.New()
	testUserID := uuid.New()
	testRoleID := uuid.New()
	testJoinedAt := time.Now().UTC().Truncate(time.Second)

	type args struct {
		accountID   uuid.UUID
		initiatorID uuid.UUID
		groupID     uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.AccessMock,
			*repository_mocks.GroupMemberMock,
			*service_mocks.UserMock,
			*service_mocks.GroupRoleMock,
		)
		args    args
		want    []domain.GroupMemberDetails
		wantErr error
	}{
		{
			name: "no access is forbidden",
			setupMocks: func(
				access *service_mocks.AccessMock,
				_ *repository_mocks.GroupMemberMock,
				_ *service_mocks.UserMock,
				_ *service_mocks.GroupRoleMock,
			) {
				access.IsCheckGroupActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID, testInitiatorID, testGroupID,
						domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
					).Return(service.ErrForbidden)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID},
			want:    nil,
			wantErr: service.ErrForbidden,
		},
		{
			name: "no members returns empty slice",
			setupMocks: func(
				access *service_mocks.AccessMock,
				repo *repository_mocks.GroupMemberMock,
				_ *service_mocks.UserMock,
				_ *service_mocks.GroupRoleMock,
			) {
				access.IsCheckGroupActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID, testInitiatorID, testGroupID,
						domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
					).Return(nil)
				repo.SelectByGroupIDMock.Expect(minimock.AnyContext, testGroupID).
					Return([]domain.GroupMember{}, nil)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID},
			want:    []domain.GroupMemberDetails{},
			wantErr: nil,
		},
		{
			name: "success",
			setupMocks: func(
				access *service_mocks.AccessMock,
				repo *repository_mocks.GroupMemberMock,
				user *service_mocks.UserMock,
				groupRole *service_mocks.GroupRoleMock,
			) {
				access.IsCheckGroupActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID, testInitiatorID, testGroupID,
						domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
					).Return(nil)
				repo.SelectByGroupIDMock.Expect(minimock.AnyContext, testGroupID).
					Return([]domain.GroupMember{
						{GroupID: testGroupID, UserID: testUserID, RoleID: testRoleID, JoinedAt: testJoinedAt},
					}, nil)
				user.GetByIDsMock.Expect(minimock.AnyContext, []uuid.UUID{testUserID}).
					Return([]domain.User{{ID: testUserID, Name: "Иван", Surname: "Иванов", Email: "ivan@example.com"}}, nil)
				groupRole.GetByAccountIDMock.Expect(minimock.AnyContext, testAccountID).
					Return([]domain.GroupRole{
						{
							ID:             testRoleID,
							Name:           "Менеджер",
							PermissionMask: domain.SetBits(0, domain.GroupPermissionManageMembers),
						},
					}, nil)
			},
			args: args{testAccountID, testInitiatorID, testGroupID},
			want: []domain.GroupMemberDetails{
				{
					UserID:         testUserID,
					Name:           "Иван",
					Surname:        "Иванов",
					Email:          "ivan@example.com",
					RoleID:         testRoleID,
					RoleName:       "Менеджер",
					PermissionMask: domain.SetBits(0, domain.GroupPermissionManageMembers),
					JoinedAt:       testJoinedAt,
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Access, mockRepos.GroupMember, mockServices.User, mockServices.GroupRole)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewGroupMemberService(r.GroupMember, s)

					got, err := srv.ListByGroup(t.Context(), tt.args.accountID, tt.args.initiatorID, tt.args.groupID)

					require.Equal(t, tt.want, got)
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

// TestService_GroupMember_UpdateRole покрывает смену роли участника группы (§3.3 дизайна
// эпика Э2, П-4): право — Access.IsCheckGroupAction(ManageGroups, GroupPermissionManageMembers);
// роль должна принадлежать accountID, иначе ErrNotFound (не раскрываем чужие роли); участник
// не найден — ErrNotFound.
func TestService_GroupMember_UpdateRole(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testGroupID := uuid.New()
	testUserID := uuid.New()
	testRoleID := uuid.New()
	testJoinedAt := time.Now().UTC().Truncate(time.Second)

	type args struct {
		accountID   uuid.UUID
		initiatorID uuid.UUID
		groupID     uuid.UUID
		userID      uuid.UUID
		roleID      uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.AccessMock,
			*repository_mocks.GroupMemberMock,
			*service_mocks.GroupRoleMock,
			*service_mocks.UserMock,
		)
		args    args
		want    domain.GroupMemberDetails
		wantErr error
	}{
		{
			name: "no access is forbidden",
			setupMocks: func(
				access *service_mocks.AccessMock,
				_ *repository_mocks.GroupMemberMock,
				_ *service_mocks.GroupRoleMock,
				_ *service_mocks.UserMock,
			) {
				access.IsCheckGroupActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID, testInitiatorID, testGroupID,
						domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
					).Return(service.ErrForbidden)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID, testUserID, testRoleID},
			want:    domain.GroupMemberDetails{},
			wantErr: service.ErrForbidden,
		},
		{
			name: "role from another account is not found",
			setupMocks: func(
				access *service_mocks.AccessMock,
				_ *repository_mocks.GroupMemberMock,
				groupRole *service_mocks.GroupRoleMock,
				_ *service_mocks.UserMock,
			) {
				access.IsCheckGroupActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID, testInitiatorID, testGroupID,
						domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
					).Return(nil)
				groupRole.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.GroupRole{{ID: testRoleID, AccountID: uuid.New()}}, nil)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID, testUserID, testRoleID},
			want:    domain.GroupMemberDetails{},
			wantErr: service.ErrNotFound,
		},
		{
			name: "member not found",
			setupMocks: func(
				access *service_mocks.AccessMock,
				repo *repository_mocks.GroupMemberMock,
				groupRole *service_mocks.GroupRoleMock,
				_ *service_mocks.UserMock,
			) {
				access.IsCheckGroupActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID, testInitiatorID, testGroupID,
						domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
					).Return(nil)
				groupRole.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.GroupRole{{ID: testRoleID, AccountID: testAccountID}}, nil)
				repo.UpdateRoleMock.Expect(minimock.AnyContext, testGroupID, testUserID, testRoleID).
					Return(domain.GroupMember{}, repository.ErrNotFound)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID, testUserID, testRoleID},
			want:    domain.GroupMemberDetails{},
			wantErr: repository.ErrNotFound,
		},
		{
			name: "success",
			setupMocks: func(
				access *service_mocks.AccessMock,
				repo *repository_mocks.GroupMemberMock,
				groupRole *service_mocks.GroupRoleMock,
				user *service_mocks.UserMock,
			) {
				access.IsCheckGroupActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID, testInitiatorID, testGroupID,
						domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
					).Return(nil)
				groupRole.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.GroupRole{{
						ID:             testRoleID,
						AccountID:      testAccountID,
						Name:           "Менеджер",
						PermissionMask: domain.SetBits(0, domain.GroupPermissionManageMembers),
					}}, nil)
				repo.UpdateRoleMock.Expect(minimock.AnyContext, testGroupID, testUserID, testRoleID).
					Return(domain.GroupMember{
						GroupID: testGroupID, UserID: testUserID, RoleID: testRoleID, JoinedAt: testJoinedAt,
					}, nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, Name: "Иван", Surname: "Иванов", Email: "ivan@example.com"}}, nil)
			},
			args: args{testAccountID, testInitiatorID, testGroupID, testUserID, testRoleID},
			want: domain.GroupMemberDetails{
				UserID:         testUserID,
				Name:           "Иван",
				Surname:        "Иванов",
				Email:          "ivan@example.com",
				RoleID:         testRoleID,
				RoleName:       "Менеджер",
				PermissionMask: domain.SetBits(0, domain.GroupPermissionManageMembers),
				JoinedAt:       testJoinedAt,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Access, mockRepos.GroupMember, mockServices.GroupRole, mockServices.User)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewGroupMemberService(r.GroupMember, s)

					got, err := srv.UpdateRole(
						t.Context(),
						tt.args.accountID, tt.args.initiatorID, tt.args.groupID, tt.args.userID, tt.args.roleID,
					)

					require.Equal(t, tt.want, got)
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
