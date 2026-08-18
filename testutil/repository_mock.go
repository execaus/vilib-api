package testutil

import (
	"vilib-api/internal/repository"
	mock_repository "vilib-api/internal/repository/repository_mocks"
)

type RepositoryMock struct {
	Account               *mock_repository.AccountMock
	User                  *mock_repository.UserMock
	AccountRole           *mock_repository.AccountRoleMock
	UserGroup             *mock_repository.UserGroupMock
	GroupRole             *mock_repository.GroupRoleMock
	Video                 *mock_repository.VideoMock
	VideoAsset            *mock_repository.VideoAssetMock
	GroupMember           *mock_repository.GroupMemberMock
	Outbox                *mock_repository.OutboxMock
	PasswordResetToken    *mock_repository.PasswordResetTokenMock
	WatchProgress         *mock_repository.WatchProgressMock
	WatchSession          *mock_repository.WatchSessionMock
	AssignmentParticipant *mock_repository.AssignmentParticipantMock
}

func (r *RepositoryMock) ToRepositories() *repository.Repository {
	return &repository.Repository{
		Account:               r.Account,
		User:                  r.User,
		AccountRole:           r.AccountRole,
		UserGroup:             r.UserGroup,
		GroupRole:             r.GroupRole,
		Video:                 r.Video,
		VideoAsset:            r.VideoAsset,
		GroupMember:           r.GroupMember,
		Outbox:                r.Outbox,
		PasswordResetToken:    r.PasswordResetToken,
		WatchProgress:         r.WatchProgress,
		WatchSession:          r.WatchSession,
		AssignmentParticipant: r.AssignmentParticipant,
	}
}
