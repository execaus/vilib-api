package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/dberrors"
	"vilib-api/internal/repository"
	"vilib-api/internal/repository/repository_mocks"
	"vilib-api/internal/saga"
	"vilib-api/internal/saga/saga_mocks"
	"vilib-api/internal/service"
	"vilib-api/internal/service/service_mocks"
	"vilib-api/testutil"

	"github.com/gojuno/minimock/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestService_UserGroup_Create(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testGroupID := uuid.New()
	testGroupName := testutil.Faker.Lorem().Word()

	var errSomeError = errors.New("some error")

	type args struct {
		accountID   uuid.UUID
		initiatorID uuid.UUID
		name        string
	}

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.AccessMock,
			*repository_mocks.UserGroupMock,
		)
		args    args
		want    domain.UserGroup
		wantErr error
	}{
		{
			// По ТЗ §6.4 создание группы требует именно ManageGroups, а не ManageUsers.
			name: "forbidden",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserGroupMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageGroups,
					).Return(service.ErrForbidden)
			},
			args:    args{testAccountID, testInitiatorID, testGroupName},
			want:    domain.UserGroup{},
			wantErr: service.ErrForbidden,
		},
		{
			name: "success",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserGroupMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageGroups,
					).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testGroupName).
					Return(domain.UserGroup{ID: testGroupID}, nil)
			},
			args:    args{testAccountID, testInitiatorID, testGroupName},
			want:    domain.UserGroup{ID: testGroupID},
			wantErr: nil,
		},
		{
			name: "insert error propagates",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserGroupMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageGroups,
					).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testGroupName).
					Return(domain.UserGroup{}, errSomeError)
			},
			args:    args{testAccountID, testInitiatorID, testGroupName},
			want:    domain.UserGroup{},
			wantErr: errSomeError,
		},
		{
			name: "duplicate group name returns conflict",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserGroupMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageGroups,
					).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testGroupName).
					Return(domain.UserGroup{}, dberrors.UserGroupErrors.ErrUniqueUserGroupsNameAccountIdKey)
			},
			args:    args{testAccountID, testInitiatorID, testGroupName},
			want:    domain.UserGroup{},
			wantErr: service.NewConflictErrorCode("conflict.group_name", "group name already exists"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Access, mockRepos.UserGroup)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewUserGroupService(r.UserGroup, s)

					got, err := srv.Create(t.Context(), tt.args.accountID, tt.args.initiatorID, tt.args.name)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}

// TestService_UserGroup_AddMembers покрывает право на добавление участников через общий
// примитив Access.IsCheckGroupAction (§3.1 дизайна эпика Э2, В-25) и отдельную проверку, что
// все добавляемые пользователи состоят в том же аккаунте, что и группа.
func TestService_UserGroup_AddMembers(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testGroupID := uuid.New()
	testTargetUserID := uuid.New()
	testTargetRoleID := uuid.New()
	testDefaultGroupRoleID := uuid.New()

	targetUsers := []domain.User{{ID: testTargetUserID, RoleID: testTargetRoleID}}
	targetAccountRoles := []domain.AccountRole{{ID: testTargetRoleID, AccountID: testAccountID}}
	foreignAccountRoles := []domain.AccountRole{{ID: testTargetRoleID, AccountID: uuid.New()}}
	wantMembers := []domain.GroupMember{
		{GroupID: testGroupID, UserID: testTargetUserID, RoleID: testDefaultGroupRoleID},
	}

	type args struct {
		accountID   uuid.UUID
		initiatorID uuid.UUID
		groupID     uuid.UUID
		targetsID   []uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(
			*repository_mocks.UserGroupMock,
			*service_mocks.UserMock,
			*service_mocks.AccountRoleMock,
			*service_mocks.AccessMock,
			*service_mocks.GroupMemberMock,
			*service_mocks.GroupRoleMock,
			*service_mocks.AssignmentMock,
		)
		args    args
		want    []domain.GroupMember
		wantErr error
	}{
		{
			name: "no access is forbidden",
			setupMocks: func(
				repo *repository_mocks.UserGroupMock,
				user *service_mocks.UserMock,
				accRole *service_mocks.AccountRoleMock,
				access *service_mocks.AccessMock,
				groupMember *service_mocks.GroupMemberMock,
				groupRole *service_mocks.GroupRoleMock,
				_ *service_mocks.AssignmentMock,
			) {
				access.IsCheckGroupActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID, testInitiatorID, testGroupID,
						domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
					).Return(service.ErrForbidden)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID, []uuid.UUID{testTargetUserID}},
			want:    nil,
			wantErr: service.ErrForbidden,
		},
		{
			name: "account owner adds members without group membership",
			setupMocks: func(
				repo *repository_mocks.UserGroupMock,
				user *service_mocks.UserMock,
				accRole *service_mocks.AccountRoleMock,
				access *service_mocks.AccessMock,
				groupMember *service_mocks.GroupMemberMock,
				groupRole *service_mocks.GroupRoleMock,
				assignment *service_mocks.AssignmentMock,
			) {
				access.IsCheckGroupActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID, testInitiatorID, testGroupID,
						domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
					).Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testTargetUserID).
					Return(targetUsers, nil)
				accRole.GetByIDMock.Expect(minimock.AnyContext, testTargetRoleID).
					Return(targetAccountRoles, nil)
				groupRole.GetDefaultMock.Expect(minimock.AnyContext, testAccountID).
					Return(domain.GroupRole{ID: testDefaultGroupRoleID}, nil)
				groupMember.CreateMock.
					Expect(minimock.AnyContext, testGroupID, testDefaultGroupRoleID, testTargetUserID).
					Return(wantMembers, nil)
				// Каскад обязательного обучения: новички зачисляются в назначения группы (Э3-Т3).
				assignment.OnMembersAddedMock.
					Expect(minimock.AnyContext, testGroupID, []uuid.UUID{testTargetUserID}).
					Return(nil)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID, []uuid.UUID{testTargetUserID}},
			want:    wantMembers,
			wantErr: nil,
		},
		{
			name: "target user from another account is forbidden",
			setupMocks: func(
				repo *repository_mocks.UserGroupMock,
				user *service_mocks.UserMock,
				accRole *service_mocks.AccountRoleMock,
				access *service_mocks.AccessMock,
				groupMember *service_mocks.GroupMemberMock,
				groupRole *service_mocks.GroupRoleMock,
				_ *service_mocks.AssignmentMock,
			) {
				access.IsCheckGroupActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID, testInitiatorID, testGroupID,
						domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
					).Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testTargetUserID).
					Return(targetUsers, nil)
				accRole.GetByIDMock.Expect(minimock.AnyContext, testTargetRoleID).
					Return(foreignAccountRoles, nil)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID, []uuid.UUID{testTargetUserID}},
			want:    nil,
			wantErr: service.ErrForbidden,
		},
		{
			name: "group member with ManageMembers adds members",
			setupMocks: func(
				repo *repository_mocks.UserGroupMock,
				user *service_mocks.UserMock,
				accRole *service_mocks.AccountRoleMock,
				access *service_mocks.AccessMock,
				groupMember *service_mocks.GroupMemberMock,
				groupRole *service_mocks.GroupRoleMock,
				assignment *service_mocks.AssignmentMock,
			) {
				access.IsCheckGroupActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID, testInitiatorID, testGroupID,
						domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
					).Return(nil)
				user.GetByIDMock.Expect(minimock.AnyContext, testTargetUserID).
					Return(targetUsers, nil)
				accRole.GetByIDMock.Expect(minimock.AnyContext, testTargetRoleID).
					Return(targetAccountRoles, nil)
				groupRole.GetDefaultMock.Expect(minimock.AnyContext, testAccountID).
					Return(domain.GroupRole{ID: testDefaultGroupRoleID}, nil)
				groupMember.CreateMock.
					Expect(minimock.AnyContext, testGroupID, testDefaultGroupRoleID, testTargetUserID).
					Return(wantMembers, nil)
				// Каскад обязательного обучения: новички зачисляются в назначения группы (Э3-Т3).
				assignment.OnMembersAddedMock.
					Expect(minimock.AnyContext, testGroupID, []uuid.UUID{testTargetUserID}).
					Return(nil)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID, []uuid.UUID{testTargetUserID}},
			want:    wantMembers,
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
						mockRepos.UserGroup,
						mockServices.User,
						mockServices.AccountRole,
						mockServices.Access,
						mockServices.GroupMember,
						mockServices.GroupRole,
						mockServices.Assignment,
					)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewUserGroupService(r.UserGroup, s)

					got, err := srv.AddMembers(
						t.Context(),
						tt.args.accountID,
						tt.args.initiatorID,
						tt.args.groupID,
						tt.args.targetsID...)

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

// TestService_UserGroup_Delete проверяет удаление группы (Э1-Т21, §7.3 дизайна эпика):
// проверку прав, каскадное удаление в БД и best-effort очистку объектов хранилища каждого
// видео группы после коммита транзакции.
func TestService_UserGroup_Delete(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testGroupID := uuid.New()
	testBucket := "vilib"

	t.Run("forbidden", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		access := service_mocks.NewAccessMock(mc)
		access.IsCheckAccountActionMock.
			Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
			Return(service.ErrForbidden)

		repo := repository_mocks.NewUserGroupMock(mc)
		svc := &service.Service{Access: access}
		groupSvc := service.NewUserGroupService(repo, svc)

		err := groupSvc.Delete(t.Context(), testInitiatorID, testAccountID, testGroupID)

		require.ErrorIs(t, err, service.ErrForbidden)
	})

	t.Run("repository error propagates", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		access := service_mocks.NewAccessMock(mc)
		access.IsCheckAccountActionMock.
			Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
			Return(nil)

		repoErr := errors.New("db unavailable")
		repo := repository_mocks.NewUserGroupMock(mc)
		repo.DeleteCascadeMock.Expect(minimock.AnyContext, testGroupID).Return(nil, repoErr)

		assignment := service_mocks.NewAssignmentMock(mc)
		assignment.OnGroupDeletedMock.Expect(minimock.AnyContext, testGroupID).Return(nil)

		svc := &service.Service{Access: access, Assignment: assignment}
		groupSvc := service.NewUserGroupService(repo, svc)

		err := groupSvc.Delete(t.Context(), testInitiatorID, testAccountID, testGroupID)

		require.ErrorIs(t, err, repoErr)
	})

	t.Run("success - schedules storage cleanup for each deleted video after commit", func(t *testing.T) {
		t.Parallel()

		mc := minimock.NewController(t)
		access := service_mocks.NewAccessMock(mc)
		access.IsCheckAccountActionMock.
			Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
			Return(nil)

		videoID1, videoID2 := uuid.New(), uuid.New()
		userGroupRepo := repository_mocks.NewUserGroupMock(mc)
		userGroupRepo.DeleteCascadeMock.
			Expect(minimock.AnyContext, testGroupID).
			Return([]uuid.UUID{videoID1, videoID2}, nil)

		s3Mock := service_mocks.NewS3Mock(mc)
		cleanedPrefixes := make(map[string]struct{})
		var mu sync.Mutex
		s3Mock.DeleteByPrefixMock.Set(func(_ context.Context, bucket, prefix string) (int, error) {
			require.Equal(t, testBucket, bucket)
			mu.Lock()
			cleanedPrefixes[prefix] = struct{}{}
			mu.Unlock()
			return 0, nil
		})

		videoRepo := repository_mocks.NewVideoMock(mc)
		assignment := service_mocks.NewAssignmentMock(mc)
		assignment.OnGroupDeletedMock.Expect(minimock.AnyContext, testGroupID).Return(nil)

		svc := &service.Service{Access: access, Assignment: assignment}
		svc.Video = service.NewVideoService(s3Mock, videoRepo, svc, service.VideoServiceConfig{Bucket: testBucket})
		svc.UserGroup = service.NewUserGroupService(userGroupRepo, svc)

		tx := saga_mocks.NewBobTransactionMock(mc)
		tx.CommitMock.Return(nil)
		tx.RollbackMock.Return(nil)
		tx.RollbackMock.Optional()

		txRepo := saga_mocks.NewTransactableMock(mc)
		txRepo.WithTxMock.Return(tx, nil)

		runner := saga.NewSagaRunner(svc, txRepo)

		err := runner.Run(t.Context(), func(ctx context.Context, s *service.Service) error {
			return s.UserGroup.Delete(ctx, testInitiatorID, testAccountID, testGroupID)
		})

		require.NoError(t, err)
		require.Equal(t, map[string]struct{}{
			"videos/" + videoID1.String() + "/": {},
			"videos/" + videoID2.String() + "/": {},
		}, cleanedPrefixes)
	})
}

// TestService_UserGroup_Get покрывает карточку группы (§3.2 дизайна эпика Э2, П-3): право —
// любой участник аккаунта (IsHasUser, как список групп); группа не в аккаунте — ErrNotFound.
func TestService_UserGroup_Get(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testGroupID := uuid.New()

	type args struct {
		initiatorID uuid.UUID
		accountID   uuid.UUID
		groupID     uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(*service_mocks.AccountMock, *repository_mocks.UserGroupMock)
		args       args
		want       domain.UserGroup
		wantErr    error
	}{
		{
			name: "initiator not in account is forbidden",
			setupMocks: func(acc *service_mocks.AccountMock, _ *repository_mocks.UserGroupMock) {
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(service.ErrForbidden)
			},
			args:    args{testInitiatorID, testAccountID, testGroupID},
			want:    domain.UserGroup{},
			wantErr: service.ErrForbidden,
		},
		{
			name: "group not found",
			setupMocks: func(acc *service_mocks.AccountMock, repo *repository_mocks.UserGroupMock) {
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(nil)
				repo.GetByIDMock.Expect(minimock.AnyContext, testGroupID).
					Return(nil, repository.ErrNotFound)
			},
			args:    args{testInitiatorID, testAccountID, testGroupID},
			want:    domain.UserGroup{},
			wantErr: service.ErrNotFound,
		},
		{
			name: "group from another account is not found",
			setupMocks: func(acc *service_mocks.AccountMock, repo *repository_mocks.UserGroupMock) {
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(nil)
				repo.GetByIDMock.Expect(minimock.AnyContext, testGroupID).
					Return([]domain.UserGroup{{ID: testGroupID, AccountID: uuid.New()}}, nil)
			},
			args:    args{testInitiatorID, testAccountID, testGroupID},
			want:    domain.UserGroup{},
			wantErr: service.ErrNotFound,
		},
		{
			name: "success",
			setupMocks: func(acc *service_mocks.AccountMock, repo *repository_mocks.UserGroupMock) {
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(nil)
				repo.GetByIDMock.Expect(minimock.AnyContext, testGroupID).
					Return([]domain.UserGroup{{ID: testGroupID, AccountID: testAccountID, Name: "test"}}, nil)
			},
			args:    args{testInitiatorID, testAccountID, testGroupID},
			want:    domain.UserGroup{ID: testGroupID, AccountID: testAccountID, Name: "test"},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Account, mockRepos.UserGroup)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewUserGroupService(r.UserGroup, s)

					got, err := srv.Get(t.Context(), tt.args.initiatorID, tt.args.accountID, tt.args.groupID)

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

// TestService_UserGroup_Rename проверяет переименование группы (§4 дизайна эпика Э2, «Блок
// C — редактирование»): право ManageGroups, группа должна принадлежать accountID, дубль имени
// — ErrGroupNameExists.
func TestService_UserGroup_Rename(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testGroupID := uuid.New()
	testNewName := testutil.Faker.Lorem().Word()

	type args struct {
		initiatorID uuid.UUID
		accountID   uuid.UUID
		groupID     uuid.UUID
		name        string
	}

	tests := []struct {
		name       string
		setupMocks func(*service_mocks.AccessMock, *repository_mocks.UserGroupMock)
		args       args
		want       domain.UserGroup
		wantErr    error
	}{
		{
			name: "forbidden",
			setupMocks: func(access *service_mocks.AccessMock, _ *repository_mocks.UserGroupMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(service.ErrForbidden)
			},
			args:    args{testInitiatorID, testAccountID, testGroupID, testNewName},
			want:    domain.UserGroup{},
			wantErr: service.ErrForbidden,
		},
		{
			name: "group not found",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserGroupMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(nil)
				repo.GetByIDMock.Expect(minimock.AnyContext, testGroupID).Return(nil, repository.ErrNotFound)
			},
			args:    args{testInitiatorID, testAccountID, testGroupID, testNewName},
			want:    domain.UserGroup{},
			wantErr: service.ErrNotFound,
		},
		{
			name: "group from another account is not found",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserGroupMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(nil)
				repo.GetByIDMock.Expect(minimock.AnyContext, testGroupID).
					Return([]domain.UserGroup{{ID: testGroupID, AccountID: uuid.New()}}, nil)
			},
			args:    args{testInitiatorID, testAccountID, testGroupID, testNewName},
			want:    domain.UserGroup{},
			wantErr: service.ErrNotFound,
		},
		{
			name: "duplicate group name returns conflict",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserGroupMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(nil)
				repo.GetByIDMock.Expect(minimock.AnyContext, testGroupID).
					Return([]domain.UserGroup{{ID: testGroupID, AccountID: testAccountID}}, nil)
				repo.UpdateNameMock.Expect(minimock.AnyContext, testGroupID, testNewName).
					Return(domain.UserGroup{}, dberrors.UserGroupErrors.ErrUniqueUserGroupsNameAccountIdKey)
			},
			args:    args{testInitiatorID, testAccountID, testGroupID, testNewName},
			want:    domain.UserGroup{},
			wantErr: service.ErrGroupNameExists,
		},
		{
			name: "success",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserGroupMock) {
				access.IsCheckAccountActionMock.
					Expect(minimock.AnyContext, testAccountID, testInitiatorID, domain.AccountPermissionManageGroups).
					Return(nil)
				repo.GetByIDMock.Expect(minimock.AnyContext, testGroupID).
					Return([]domain.UserGroup{{ID: testGroupID, AccountID: testAccountID}}, nil)
				repo.UpdateNameMock.Expect(minimock.AnyContext, testGroupID, testNewName).
					Return(domain.UserGroup{ID: testGroupID, AccountID: testAccountID, Name: testNewName}, nil)
			},
			args: args{testInitiatorID, testAccountID, testGroupID, testNewName},
			want: domain.UserGroup{ID: testGroupID, AccountID: testAccountID, Name: testNewName},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Access, mockRepos.UserGroup)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewUserGroupService(r.UserGroup, s)

					got, err := srv.Rename(
						t.Context(),
						tt.args.initiatorID,
						tt.args.accountID,
						tt.args.groupID,
						tt.args.name,
					)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}
