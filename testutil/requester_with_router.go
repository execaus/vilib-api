package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
)

type Requester struct {
	method       string
	target       string
	body         any
	localMailBox chan string
	version      string
	router       *gin.Engine
	t            *testing.T
}

// RequestWithRouter инициализирует Requester с заданным Gin-роутером и версией API.
// Используется в тестах для формирования и выполнения HTTP-запросов к переданному роутеру.
func RequestWithRouter(t *testing.T, apiVersion string, router *gin.Engine) *Requester {
	return &Requester{
		t:       t,
		router:  router,
		version: apiVersion,
	}
}

func (r *Requester) Method(method string) *Requester {
	r.method = method
	return r
}

func (r *Requester) Target(target string) *Requester {
	r.target = target
	return r
}

func (r *Requester) Body(body any) *Requester {
	r.body = body
	return r
}

func (r *Requester) LocalMailBox(localMailBox chan string) *Requester {
	r.localMailBox = localMailBox
	return r
}

func (r *Requester) Version(version string) *Requester {
	r.version = version
	return r
}

func (r *Requester) Run(response any) (status int) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(r.t)
	defer ctrl.Finish()

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

	r.router.ServeHTTP(recorder, req)

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
