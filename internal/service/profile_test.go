package service_test

import (
	"errors"
	"testing"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
	"vilib-api/internal/service"
	"vilib-api/internal/service/service_mocks"
	"vilib-api/testutil"

	"github.com/gojuno/minimock/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestService_Profile_Get проверяет сборку контекста /me (§2.3 дизайна эпика Э2): успех
// владельца с группами, деактивированная строка (403 forbidden.user_deactivated) и отсутствие
// членств (пустой срез групп, не nil).
func TestService_Profile_Get(t *testing.T) {
	t.Parallel()

	testUserID := uuid.New()
	testRoleID := uuid.New()
	testAccountID := uuid.New()
	testOtherAccountID := uuid.New()
	testGroupID := uuid.New()
	testGroupRoleID := uuid.New()
	testEmail := testutil.Faker.Person().Contact().Email

	var errSomeError = errors.New("some error")

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.UserMock,
			*service_mocks.AccountRoleMock,
			*service_mocks.AccountMock,
			*service_mocks.GroupMemberMock,
			*service_mocks.UserGroupMock,
			*service_mocks.GroupRoleMock,
		)
		want    domain.Profile
		wantErr error
	}{
		{
			name: "get user error",
			setupMocks: func(
				user *service_mocks.UserMock, _ *service_mocks.AccountRoleMock, _ *service_mocks.AccountMock,
				_ *service_mocks.GroupMemberMock, _ *service_mocks.UserGroupMock, _ *service_mocks.GroupRoleMock,
			) {
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return(nil, errSomeError)
			},
			wantErr: errSomeError,
		},
		{
			name: "user deactivated",
			setupMocks: func(
				user *service_mocks.UserMock, _ *service_mocks.AccountRoleMock, _ *service_mocks.AccountMock,
				_ *service_mocks.GroupMemberMock, _ *service_mocks.UserGroupMock, _ *service_mocks.GroupRoleMock,
			) {
				deactivatedAt := time.Now()
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, RoleID: testRoleID, DeactivatedAt: &deactivatedAt}}, nil)
			},
			wantErr: service.ErrForbiddenUserDeactivated,
		},
		{
			name: "get role error",
			setupMocks: func(
				user *service_mocks.UserMock, role *service_mocks.AccountRoleMock, _ *service_mocks.AccountMock,
				_ *service_mocks.GroupMemberMock, _ *service_mocks.UserGroupMock, _ *service_mocks.GroupRoleMock,
			) {
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, RoleID: testRoleID}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return(nil, errSomeError)
			},
			wantErr: errSomeError,
		},
		{
			name: "no memberships returns empty groups slice, not nil",
			setupMocks: func(
				user *service_mocks.UserMock, role *service_mocks.AccountRoleMock, acc *service_mocks.AccountMock,
				member *service_mocks.GroupMemberMock, _ *service_mocks.UserGroupMock, _ *service_mocks.GroupRoleMock,
			) {
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, RoleID: testRoleID, Email: testEmail}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{
						ID: testRoleID, AccountID: testAccountID,
						PermissionMask: domain.SetBits(0, domain.AccountPermissionOwner),
					}}, nil)
				acc.GetByIDMock.Expect(minimock.AnyContext, testAccountID).
					Return([]domain.Account{{ID: testAccountID, Name: "acme"}}, nil)
				acc.GetByUserEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.Account{{ID: testAccountID, Name: "acme"}}, nil)
				member.GetByUserIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.GroupMember{}, nil)
			},
			want: domain.Profile{
				User:     domain.User{ID: testUserID, RoleID: testRoleID, Email: testEmail},
				Account:  domain.Account{ID: testAccountID, Name: "acme"},
				Accounts: []domain.Account{{ID: testAccountID, Name: "acme"}},
				Role: domain.AccountRole{
					ID: testRoleID, AccountID: testAccountID,
					PermissionMask: domain.SetBits(0, domain.AccountPermissionOwner),
				},
				IsOwner: true,
				Groups:  []domain.GroupMembership{},
			},
			wantErr: nil,
		},
		{
			name: "success with memberships",
			setupMocks: func(
				user *service_mocks.UserMock, role *service_mocks.AccountRoleMock, acc *service_mocks.AccountMock,
				member *service_mocks.GroupMemberMock, group *service_mocks.UserGroupMock, groupRole *service_mocks.GroupRoleMock,
			) {
				user.GetByIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.User{{ID: testUserID, RoleID: testRoleID, Email: testEmail}}, nil)
				role.GetByIDMock.Expect(minimock.AnyContext, testRoleID).
					Return([]domain.AccountRole{{ID: testRoleID, AccountID: testAccountID}}, nil)
				acc.GetByIDMock.Expect(minimock.AnyContext, testAccountID).
					Return([]domain.Account{{ID: testAccountID, Name: "acme"}}, nil)
				acc.GetByUserEmailMock.Expect(minimock.AnyContext, testEmail).
					Return([]domain.Account{{ID: testOtherAccountID}, {ID: testAccountID, Name: "acme"}}, nil)
				member.GetByUserIDMock.Expect(minimock.AnyContext, testUserID).
					Return([]domain.GroupMember{{GroupID: testGroupID, UserID: testUserID, RoleID: testGroupRoleID}}, nil)
				group.GetByIDMock.Expect(minimock.AnyContext, testGroupID).
					Return([]domain.UserGroup{{ID: testGroupID, Name: "editors", AccountID: testAccountID}}, nil)
				groupRole.GetByAccountIDMock.Expect(minimock.AnyContext, testAccountID).
					Return([]domain.GroupRole{{
						ID: testGroupRoleID, Name: "editor", AccountID: testAccountID,
						PermissionMask: domain.SetBits(0, domain.GroupPermissionManageVideo),
					}}, nil)
			},
			want: domain.Profile{
				User:     domain.User{ID: testUserID, RoleID: testRoleID, Email: testEmail},
				Account:  domain.Account{ID: testAccountID, Name: "acme"},
				Accounts: []domain.Account{{ID: testOtherAccountID}, {ID: testAccountID, Name: "acme"}},
				Role:     domain.AccountRole{ID: testRoleID, AccountID: testAccountID},
				IsOwner:  false,
				Groups: []domain.GroupMembership{
					{
						GroupID: testGroupID, GroupName: "editors",
						RoleID: testGroupRoleID, RoleName: "editor",
						PermissionMask: domain.SetBits(0, domain.GroupPermissionManageVideo),
					},
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
				func(mockServices *testutil.ServiceMock, _ *testutil.RepositoryMock) {
					tt.setupMocks(
						mockServices.User,
						mockServices.AccountRole,
						mockServices.Account,
						mockServices.GroupMember,
						mockServices.UserGroup,
						mockServices.GroupRole,
					)
				},
				func(s *service.Service, _ *repository.Repository) {
					srv := service.NewProfileService(s)

					got, err := srv.Get(t.Context(), testUserID)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}
