package end2end_test

import (
	"net/http"
	"testing"
	"vilib-api/config"
	"vilib-api/internal/dto"
	"vilib-api/internal/handler"
	"vilib-api/internal/repository"
	"vilib-api/internal/service"
	"vilib-api/server"
	"vilib-api/test/end2end"

	"github.com/stephenafamo/bob"
	"github.com/stretchr/testify/require"
)

const (
	adminEmail = "admin@mail.ru"
)

var (
	migrationsPath = []string{"../../migrations"}
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

	end2end.WithDB(t, migrationsPath, func(bobDB *bob.DB) {
		r := repository.NewTransactionalRepository(bobDB)
		s := service.NewService(cfg, localMailBox, r)
		h := handler.NewHandler(service.NewSagaRunner(s, r))
		router := h.GetRouter()

		t.Run("admin_registration", func(t *testing.T) {
			code := end2end.RequestWithRouter(t, handler.APIVersion1, router).
				Method(http.MethodPost).
				Target(handler.RegisterURL).
				Body(dto.RegisterRequest{
					Name:    "admin_name",
					Surname: "admin_surname",
					Email:   adminEmail,
				}).Run(nil)

			require.Equal(t, http.StatusCreated, code)
		})

		adminPassword := <-localMailBox

		require.NotEmpty(t, adminPassword, "admin password is not created")

		t.Run("account_name_exists_error", func(t *testing.T) {
			// TODO
		})
	})
}
