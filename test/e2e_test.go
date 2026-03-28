package test_test

import (
	"net/http"
	"testing"
	"vilib-api/config"
	"vilib-api/internal/dto"
	"vilib-api/internal/handler"
	"vilib-api/internal/repository"
	"vilib-api/internal/saga"
	"vilib-api/internal/service"
	"vilib-api/server"
	"vilib-api/testutil"

	"github.com/stephenafamo/bob"
	"github.com/stretchr/testify/require"
)

const (
	adminEmail = "admin@mail.ru"
)

func TestEnd2End(t *testing.T) {
	localMailBox := make(chan string, 1)
	mainScenario(t, localMailBox)
}

func mainScenario(t *testing.T, localMailBox chan string) {
	cfg := config.Config{
		Auth: config.AuthConfig{
			Key: "key",
		},
		Server: config.ServerConfig{
			Origin: "",
			Port:   "",
			Mode:   server.DevelopmentMode,
		},
	}

	testutil.WithDB(t, testutil.MigrationsPath, func(bobDB *bob.DB) {
		provider := repository.NewExecutorProvider(bobDB)
		r := repository.NewRepository(provider)
		s := service.NewService(cfg, localMailBox, r)
		h := handler.NewHandler(saga.NewSagaRunner(s, provider))
		router := h.GetRouter()

		var (
			adminPassword string
			adminToken    string
		)

		t.Run("admin_registration", func(t *testing.T) {
			code := testutil.RequestWithRouter(t, handler.APIVersion1, router).
				Method(http.MethodPost).
				Target(handler.RegisterURL).
				Body(dto.RegisterRequest{
					Name:    "admin_name",
					Surname: "admin_surname",
					Email:   adminEmail,
				}).Run(nil)

			require.Equal(t, http.StatusCreated, code)
		})

		adminPassword = <-localMailBox

		require.NotEmpty(t, adminPassword, "admin password is not created")

		t.Run("admin_authorization", func(t *testing.T) {
			var response dto.LoginResponse

			code := testutil.RequestWithRouter(t, handler.APIVersion1, router).
				Method(http.MethodPost).
				Target(handler.LoginURL).
				Body(dto.LoginRequest{
					Email:    adminEmail,
					Password: adminPassword,
				}).Run(&response)

			require.Equal(t, http.StatusOK, code)
			require.NotEmpty(t, response.Token)

			adminToken = response.Token

			_ = adminToken
		})

		// TODO получение данных о текущем аккаунте

		//t.Run("admin_create_user", func(t *testing.T) {
		//	var response dto.CreateUserResponse
		//
		//	code := testutil.RequestWithRouter(t, handler.APIVersion1, router).
		//		Method(http.MethodPost).
		//		Target(handler.CreateUserURL.WithValues(accountID)).
		//		Body(dto.LoginRequest{
		//			Email:    adminEmail,
		//			Password: adminPassword,
		//		}).Run(&response)
		//
		//	require.Equal(t, http.StatusCreated, code)
		//	// проверить отправку пароля новому пользователю
		//	// проверить права нового пользователя - только user
		//	// попробовать авторизоваться под новым пользователем
		//})
	})
}
