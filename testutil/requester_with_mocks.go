package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"vilib-api/internal/handler"
	"vilib-api/internal/saga"
	mock_saga "vilib-api/internal/saga/saga_mocks"
	mock_service "vilib-api/internal/service/service_mocks"

	"github.com/gin-gonic/gin"
	"github.com/gojuno/minimock/v3"
)

type RequesterWithMocks struct {
	method         string
	target         string
	body           any
	authToken      string
	localMailBox   chan string
	prepareService func(t *testing.T, service *ServiceMock)
	version        string
	t              *testing.T
}

// RequestWithMocks инициализирует RequesterWithMocks с заданной версией API.
// Используется в тестах для выполнения HTTP-запросов с подменёнными сервисами (моками).
func RequestWithMocks(t *testing.T, apiVersion string) *RequesterWithMocks {
	return &RequesterWithMocks{
		version: apiVersion,
		t:       t,
	}
}

func (r *RequesterWithMocks) Method(method string) *RequesterWithMocks {
	r.method = method
	return r
}

func (r *RequesterWithMocks) Target(target string) *RequesterWithMocks {
	r.target = target
	return r
}

func (r *RequesterWithMocks) Body(body any) *RequesterWithMocks {
	r.body = body
	return r
}

func (r *RequesterWithMocks) Authorization(token string) *RequesterWithMocks {
	r.authToken = token
	return r
}

func (r *RequesterWithMocks) LocalMailBox(localMailBox chan string) *RequesterWithMocks {
	r.localMailBox = localMailBox
	return r
}

func (r *RequesterWithMocks) PrepareService(
	prepareService func(t *testing.T, service *ServiceMock),
) *RequesterWithMocks {
	r.prepareService = prepareService
	return r
}

func (r *RequesterWithMocks) Version(version string) *RequesterWithMocks {
	r.version = version
	return r
}

func (r *RequesterWithMocks) Run(response any) (status int) {
	gin.SetMode(gin.TestMode)

	ctrl := minimock.NewController(r.t)

	s := &ServiceMock{
		Auth:        mock_service.NewAuthMock(ctrl),
		User:        mock_service.NewUserMock(ctrl),
		Account:     mock_service.NewAccountMock(ctrl),
		Email:       mock_service.NewEmailMock(ctrl),
		AccountRole: mock_service.NewAccountRoleMock(ctrl),
		UserGroup:   mock_service.NewUserGroupMock(ctrl),
		GroupRole:   mock_service.NewGroupRoleMock(ctrl),
		Video:       mock_service.NewVideoMock(ctrl),
		VideoAsset:  mock_service.NewVideoAssetMock(ctrl),
	}

	if r.prepareService != nil {
		r.prepareService(r.t, s)
	}

	repo := mock_saga.NewTransactableMock(ctrl)
	tx := mock_saga.NewBobTransactionMock(ctrl)

	tx.CommitMock.Expect(minimock.AnyContext).Return(nil)
	repo.WithTxMock.Expect(minimock.AnyContext).Return(tx, nil)

	h := handler.NewHandler(saga.NewSagaRunner(s.ToServices(), repo))
	router := h.GetRouter()

	recorder := httptest.NewRecorder()

	var (
		jsonBody []byte
		err      error
	)

	if r.body != nil {
		jsonBody, err = json.Marshal(r.body)
		if err != nil {
			r.t.Fatal(err)
		}
	}

	req := httptest.NewRequest(r.method, FullURI(r.version, r.target), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	if r.authToken != "" {
		req.Header.Set("Authorization", r.authToken)
	}

	router.ServeHTTP(recorder, req)

	if recorder.Code == http.StatusInternalServerError {
		return recorder.Code
	}

	if response != nil {
		if err = json.Unmarshal(recorder.Body.Bytes(), response); err != nil {
			r.t.Fatal(err)
		}
	}

	return recorder.Code
}
