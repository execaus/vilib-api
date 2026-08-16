package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"vilib-api/internal/domain"
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
			name: "forbidden",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.UserGroupMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionManageUsers,
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
						domain.AccountPermissionManageUsers,
					).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testGroupName).
					Return(domain.UserGroup{ID: testGroupID}, nil)
			},
			args:    args{testAccountID, testInitiatorID, testGroupName},
			want:    domain.UserGroup{ID: testGroupID},
			wantErr: nil,
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

func TestService_UserGroup_AddMembers(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testGroupID := uuid.New()
	testTargetUserID := uuid.New()

	type args struct {
		accountID   uuid.UUID
		initiatorID uuid.UUID
		groupID     uuid.UUID
		targetsID   []uuid.UUID
	}

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.AccountMock,
		)
		args    args
		want    []domain.GroupMember
		wantErr error
	}{
		{
			name: "user not in account",
			setupMocks: func(acc *service_mocks.AccountMock) {
				acc.IsHasUserMock.Expect(minimock.AnyContext, testAccountID, testInitiatorID).
					Return(service.ErrForbidden)
			},
			args:    args{testAccountID, testInitiatorID, testGroupID, []uuid.UUID{testTargetUserID}},
			want:    nil,
			wantErr: service.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Account)
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
					require.Equal(t, tt.wantErr, err)
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

		svc := &service.Service{Access: access}
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
		svc := &service.Service{Access: access}
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
