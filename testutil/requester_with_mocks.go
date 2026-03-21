package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"vilib-api/internal/handler"
	mock_postgres "vilib-api/internal/repository/mocks"
	"vilib-api/internal/service"
	mock_service "vilib-api/internal/service/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
)

type RequesterWithMocks struct {
	method         string
	target         string
	body           any
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

	ctrl := gomock.NewController(r.t)
	defer ctrl.Finish()

	s := &ServiceMock{
		Auth:    mock_service.NewMockAuth(ctrl),
		Account: mock_service.NewMockAccount(ctrl),
		User:    mock_service.NewMockUser(ctrl),
		Email:   mock_service.NewMockEmail(ctrl),
	}

	if r.prepareService != nil {
		r.prepareService(r.t, s)
	}

	repo := mock_postgres.NewMockTransactable(ctrl)
	tx := mock_postgres.NewMockBobTransaction(ctrl)

	tx.EXPECT().Commit(gomock.Any()).Return(nil).AnyTimes()
	repo.EXPECT().WithTx(gomock.Any()).Return(tx, nil).AnyTimes()

	h := handler.NewHandler(service.NewSagaRunner(s.ToServices(), repo))
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
