package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegisterHandler(t *testing.T) {
	body := map[string]string{
		"email":    "test@mail.com",
		"password": "123456",
	}

	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(
		http.MethodPost,
		"/register",
		bytes.NewBuffer(jsonBody),
	)

	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	handler := http.HandlerFunc(RegisterHandler)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}
