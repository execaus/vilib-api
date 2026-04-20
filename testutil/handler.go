package testutil

import (
	"vilib-api/internal/handler"
	"vilib-api/internal/saga"
	"vilib-api/internal/saga/saga_mocks"
	"vilib-api/internal/service"
	"vilib-api/internal/service/service_mocks"

	"github.com/gin-gonic/gin"
	"github.com/gojuno/minimock/v3"
)

type HandlerTestServiceMock struct {
	Auth        *service_mocks.AuthMock
	Account     *service_mocks.AccountMock
	AccountRole *service_mocks.AccountRoleMock
	User        *service_mocks.UserMock
	Email       *service_mocks.EmailMock
	UserGroup   *service_mocks.UserGroupMock
	GroupMember *service_mocks.GroupMemberMock
	GroupRole   *service_mocks.GroupRoleMock
	Video       *service_mocks.VideoMock
	VideoAsset  *service_mocks.VideoAssetMock
	Access      *service_mocks.AccessMock
}

func NewHandlerTestServiceMock(mc *minimock.Controller) *HandlerTestServiceMock {
	return &HandlerTestServiceMock{
		Auth:        service_mocks.NewAuthMock(mc),
		Account:     service_mocks.NewAccountMock(mc),
		AccountRole: service_mocks.NewAccountRoleMock(mc),
		User:        service_mocks.NewUserMock(mc),
		Email:       service_mocks.NewEmailMock(mc),
		UserGroup:   service_mocks.NewUserGroupMock(mc),
		GroupMember: service_mocks.NewGroupMemberMock(mc),
		GroupRole:   service_mocks.NewGroupRoleMock(mc),
		Video:       service_mocks.NewVideoMock(mc),
		VideoAsset:  service_mocks.NewVideoAssetMock(mc),
		Access:      service_mocks.NewAccessMock(mc),
	}
}

func (s *HandlerTestServiceMock) ToService() *service.Service {
	return &service.Service{
		Auth:        s.Auth,
		Account:     s.Account,
		AccountRole: s.AccountRole,
		User:        s.User,
		Email:       s.Email,
		UserGroup:   s.UserGroup,
		GroupMember: s.GroupMember,
		GroupRole:   s.GroupRole,
		Video:       s.Video,
		VideoAsset:  s.VideoAsset,
		Access:      s.Access,
	}
}

func SetupTestRouterWithMocks(
	mc *minimock.Controller,
	svcMock *HandlerTestServiceMock,
	setupMocks func(*HandlerTestServiceMock),
) *gin.Engine {
	gin.SetMode(gin.TestMode)

	if setupMocks != nil {
		setupMocks(svcMock)
	}

	repo := saga_mocks.NewTransactableMock(mc)
	tx := saga_mocks.NewBobTransactionMock(mc)

	tx.CommitMock.Expect(minimock.AnyContext).Return(nil)
	repo.WithTxMock.Expect(minimock.AnyContext).Return(tx, nil)

	h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo))
	return h.GetRouter()
}

func SetupTestRouterWithoutTx(mc *minimock.Controller, svcMock *HandlerTestServiceMock) *gin.Engine {
	gin.SetMode(gin.TestMode)

	repo := saga_mocks.NewTransactableMock(mc)
	tx := saga_mocks.NewBobTransactionMock(mc)

	repo.WithTxMock.When(minimock.AnyContext).Then(tx, nil)
	repo.WithTxMock.Optional()

	h := handler.NewHandler(saga.NewSagaRunner(svcMock.ToService(), repo))
	return h.GetRouter()
}
