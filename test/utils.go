package test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"vilib-api/internal/handler"
	"vilib-api/internal/service"
	"vilib-api/internal/service/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
)

func Request(
	t *testing.T,
	method string,
	target string,
	response any,
	body any,
	prepareService func(t *testing.T, service *service.Service),
) (status int) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s := &service.Service{
		Auth: mocks.NewMockAuth(ctrl),
	}

	prepareService(t, s)

	h := handler.NewHandler(service.NewSagaRunner(s, nil))
	router := h.GetRouter()

	recorder := httptest.NewRecorder()

	var (
		jsonBody []byte
		err      error
	)

	if body != nil {
		jsonBody, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(method, target, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, req)

	if response != nil {
		if err = json.Unmarshal(recorder.Body.Bytes(), response); err != nil {
			t.Fatal(err)
		}
	}

	return recorder.Code
}
