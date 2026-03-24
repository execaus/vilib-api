package service_test

import (
	"testing"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
	"vilib-api/internal/service"
	"vilib-api/testutil"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestUserService_Update_AccessMatrix(t *testing.T) {
	const (
		userID1    = "1"
		userID2    = "2"
		accountID1 = "1"
		accountID2 = "2"
	)

	var (
		superAdminStatus = domain.SetBitsUpTo(domain.DefaultPermissionMask, domain.AccountSuperAdminBitPosition)
		adminStatus      = domain.SetBitsUpTo(domain.DefaultPermissionMask, domain.AccountAdminBitPosition)
		moderatorStatus  = domain.SetBitsUpTo(domain.DefaultPermissionMask, domain.AccountModeratorBitPosition)
		userStatus       = domain.SetBitsUpTo(domain.DefaultPermissionMask, domain.AccountUserBitPosition)
	)

	tests := []struct {
		name               string
		initiatorStatus    domain.BitmapValue
		targetStatus       domain.BitmapValue
		toStatus           domain.PermissionFlag
		initiatorID        string
		targetID           string
		initiatorAccountID string
		targetAccountID    string
		expectedErr        error
	}{
		{
			name:               "супер администратор успешно изменяет роль у пользователя",
			initiatorStatus:    superAdminStatus,
			targetStatus:       userStatus,
			toStatus:           domain.AccountModeratorBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        nil,
		},
		{
			name:               "супер администратор успешно изменяет роль у модератора",
			initiatorStatus:    superAdminStatus,
			targetStatus:       moderatorStatus,
			toStatus:           domain.AccountAdminBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        nil,
		},
		{
			name:               "супер администратор успешно изменяет роль у администратора",
			initiatorStatus:    superAdminStatus,
			targetStatus:       adminStatus,
			toStatus:           domain.AccountUserBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        nil,
		},
		{
			name:               "администратор успешно изменяет роль у пользователя",
			initiatorStatus:    adminStatus,
			targetStatus:       userStatus,
			toStatus:           domain.AccountModeratorBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        nil,
		},
		{
			name:               "администратор успешно изменяет роль у модератора",
			initiatorStatus:    adminStatus,
			targetStatus:       moderatorStatus,
			toStatus:           domain.AccountUserBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        nil,
		},
		{
			name:               "администратор не может изменить роль у администратора",
			initiatorStatus:    adminStatus,
			targetStatus:       adminStatus,
			toStatus:           domain.AccountModeratorBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        service.ErrChangeAccountStatusForbidden,
		},
		{
			name:               "администратор не может изменить роль у супер администратора",
			initiatorStatus:    adminStatus,
			targetStatus:       superAdminStatus,
			toStatus:           domain.AccountUserBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        service.ErrChangeAccountStatusForbidden,
		},
		{
			name:               "модератор успешно изменяет роль у пользователя",
			initiatorStatus:    moderatorStatus,
			targetStatus:       userStatus,
			toStatus:           domain.AccountUserBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        nil,
		},
		{
			name:               "модератор не может изменить роль у модератора",
			initiatorStatus:    moderatorStatus,
			targetStatus:       moderatorStatus,
			toStatus:           domain.AccountUserBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        service.ErrChangeAccountStatusForbidden,
		},
		{
			name:               "модератор не может изменить роль у администратора",
			initiatorStatus:    moderatorStatus,
			targetStatus:       adminStatus,
			toStatus:           domain.AccountUserBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        service.ErrChangeAccountStatusForbidden,
		},
		{
			name:               "модератор не может изменить роль у супер администратора",
			initiatorStatus:    moderatorStatus,
			targetStatus:       superAdminStatus,
			toStatus:           domain.AccountUserBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        service.ErrChangeAccountStatusForbidden,
		},
		{
			name:               "пользователь не может изменить роль у пользователя",
			initiatorStatus:    userStatus,
			targetStatus:       userStatus,
			toStatus:           domain.AccountModeratorBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        service.ErrChangeAccountStatusForbidden,
		},
		{
			name:               "пользователь не может изменить роль у модератора",
			initiatorStatus:    userStatus,
			targetStatus:       moderatorStatus,
			toStatus:           domain.AccountUserBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        service.ErrChangeAccountStatusForbidden,
		},
		{
			name:               "пользователь не может изменить роль у администратора",
			initiatorStatus:    userStatus,
			targetStatus:       adminStatus,
			toStatus:           domain.AccountUserBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        service.ErrChangeAccountStatusForbidden,
		},
		{
			name:               "пользователь не может изменить роль у супер администратора",
			initiatorStatus:    userStatus,
			targetStatus:       superAdminStatus,
			toStatus:           domain.AccountUserBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        service.ErrChangeAccountStatusForbidden,
		},
		{
			name:               "модератор не может повысить роль до модератора",
			initiatorStatus:    moderatorStatus,
			targetStatus:       userStatus,
			toStatus:           domain.AccountModeratorBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        service.ErrChangeAccountStatusForbidden,
		},
		{
			name:               "модератор не может повысить роль до администратора",
			initiatorStatus:    moderatorStatus,
			targetStatus:       userStatus,
			toStatus:           domain.AccountAdminBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        service.ErrChangeAccountStatusForbidden,
		},
		{
			name:               "модератор не может повысить роль до супер администратора",
			initiatorStatus:    moderatorStatus,
			targetStatus:       userStatus,
			toStatus:           domain.AccountSuperAdminBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        service.ErrChangeAccountStatusForbidden,
		},
		{
			name:               "администратор не может повысить роль до администратора",
			initiatorStatus:    adminStatus,
			targetStatus:       userStatus,
			toStatus:           domain.AccountAdminBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        service.ErrChangeAccountStatusForbidden,
		},
		{
			name:               "администратор не может повысить роль до супер администратора",
			initiatorStatus:    adminStatus,
			targetStatus:       userStatus,
			toStatus:           domain.AccountSuperAdminBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        service.ErrChangeAccountStatusForbidden,
		},
		{
			name:               "пользователь не может изменять статус пользователя из другого аккаунта",
			initiatorStatus:    userStatus,
			targetStatus:       userStatus,
			toStatus:           domain.AccountModeratorBitPosition,
			initiatorID:        userID1,
			targetID:           userID2,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID2,
			expectedErr:        service.ErrChangeAccountStatusForbidden,
		},
		{
			name:               "запрещено изменение своей роли",
			initiatorStatus:    superAdminStatus,
			targetStatus:       superAdminStatus,
			toStatus:           domain.AccountModeratorBitPosition,
			initiatorID:        userID1,
			targetID:           userID1,
			initiatorAccountID: accountID1,
			targetAccountID:    accountID1,
			expectedErr:        service.ErrChangeAccountStatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testutil.TestService(t, func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
				mockServices.AccountStatus.EXPECT().
					GetByUsersID(gomock.Any(), tt.initiatorID, tt.targetID).
					Return([]domain.AccountStatus{
						{
							AccountID: tt.initiatorAccountID,
							UserID:    tt.initiatorID,
							Status:    tt.initiatorStatus,
						},
						{
							AccountID: tt.targetAccountID,
							UserID:    tt.targetID,
							Status:    tt.targetStatus,
						},
					}, nil).
					AnyTimes()
				mockServices.AccountStatus.EXPECT().
					Issue(gomock.Any(), tt.targetID, tt.toStatus).
					Return(domain.AccountStatus{}, nil).
					AnyTimes()
				mockRepos.User.EXPECT().
					GetByID(gomock.Any(), tt.targetID).
					Return([]domain.User{
						{},
					}, nil).
					AnyTimes()
			}, func(s *service.Service, r *repository.Repository) {
				us := service.NewUserService(r.User, s)

				_, err := us.Update(t.Context(), tt.initiatorID, tt.targetID, &tt.toStatus)

				require.Equal(t, tt.expectedErr, err)
			})
		})
	}

	t.Run(
		"супер администратор повышает другого до супер администратора и понижается сам до администратора",
		func(t *testing.T) {
			testutil.TestService(t, func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
				mockServices.AccountStatus.EXPECT().
					GetByUsersID(gomock.Any(), userID1, userID2).
					Return([]domain.AccountStatus{
						{
							AccountID: accountID1,
							UserID:    userID1,
							Status:    superAdminStatus,
						},
						{
							AccountID: accountID1,
							UserID:    userID2,
							Status:    adminStatus,
						},
					}, nil)
				mockServices.AccountStatus.EXPECT().
					Issue(gomock.Any(), userID2, domain.AccountSuperAdminBitPosition).
					Return(domain.AccountStatus{
						AccountID: accountID1,
						UserID:    userID2,
						Status:    superAdminStatus,
					}, nil)
				mockServices.AccountStatus.EXPECT().
					Issue(gomock.Any(), userID1, domain.AccountAdminBitPosition).
					Return(domain.AccountStatus{
						AccountID: accountID1,
						UserID:    userID1,
						Status:    adminStatus,
					}, nil)
				mockRepos.User.EXPECT().
					GetByID(gomock.Any(), userID2).
					Return([]domain.User{{}}, nil)
			}, func(s *service.Service, r *repository.Repository) {
				us := service.NewUserService(r.User, s)

				sas := domain.AccountSuperAdminBitPosition
				_, err := us.Update(t.Context(), userID1, userID2, &sas)

				require.NoError(t, err)
			})
		},
	)
}
