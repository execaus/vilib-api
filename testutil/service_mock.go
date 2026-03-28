package testutil

import (
	"vilib-api/internal/service"
	mock_service "vilib-api/internal/service/service_mocks"
)

type ServiceMock struct {
	Auth        *mock_service.AuthMock
	User        *mock_service.UserMock
	Account     *mock_service.AccountMock
	Email       *mock_service.EmailMock
	AccountRole *mock_service.AccountRoleMock
	UserGroup   *mock_service.UserGroupMock
	GroupRole   *mock_service.GroupRoleMock
	Video       *mock_service.VideoMock
	VideoAsset  *mock_service.VideoAssetMock
}

func (s *ServiceMock) ToServices() *service.Service {
	return &service.Service{
		Auth:        s.Auth,
		Account:     s.Account,
		User:        s.User,
		Email:       s.Email,
		AccountRole: s.AccountRole,
		UserGroup:   s.UserGroup,
		GroupRole:   s.GroupRole,
		Video:       s.Video,
		VideoAsset:  s.VideoAsset,
	}
}
