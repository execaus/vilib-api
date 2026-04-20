package service_test

import (
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

func TestService_GroupRole_Create(t *testing.T) {
	t.Parallel()

	testAccountID := uuid.New()
	testInitiatorID := uuid.New()
	testRoleID := uuid.New()
	testName := testutil.Faker.Lorem().Word()
	testPermission := domain.PermissionMask(1)

	type args struct {
		accountID   uuid.UUID
		initiatorID uuid.UUID
		name        string
		permission  domain.PermissionMask
		isDefault   bool
	}

	tests := []struct {
		name       string
		setupMocks func(
			*service_mocks.AccessMock,
			*repository_mocks.GroupRoleMock,
		)
		args    args
		want    domain.GroupRole
		wantErr error
	}{
		{
			name: "forbidden",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.GroupRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionCreateAccountRole,
					).Return(service.ErrForbidden)
			},
			args:    args{testAccountID, testInitiatorID, testName, testPermission, false},
			want:    domain.GroupRole{},
			wantErr: service.ErrForbidden,
		},
		{
			name: "success",
			setupMocks: func(access *service_mocks.AccessMock, repo *repository_mocks.GroupRoleMock) {
				access.IsCheckAccountActionMock.
					Expect(
						minimock.AnyContext,
						testAccountID,
						testInitiatorID,
						domain.AccountPermissionCreateAccountRole,
					).Return(nil)
				repo.InsertMock.Expect(minimock.AnyContext, testAccountID, testName, testPermission, false).
					Return(domain.GroupRole{ID: testRoleID}, nil)
			},
			args:    args{testAccountID, testInitiatorID, testName, testPermission, false},
			want:    domain.GroupRole{ID: testRoleID},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testutil.TestService(
				t,
				func(mockServices *testutil.ServiceMock, mockRepos *testutil.RepositoryMock) {
					tt.setupMocks(mockServices.Access, mockRepos.GroupRole)
				},
				func(s *service.Service, r *repository.Repository) {
					srv := service.NewGroupRoleService(r.GroupRole, s)

					got, err := srv.Create(t.Context(), tt.args.accountID, tt.args.initiatorID, tt.args.name, tt.args.permission, tt.args.isDefault)

					require.Equal(t, tt.want, got)
					require.Equal(t, tt.wantErr, err)
				},
			)
		})
	}
}
