package end2end_test

import (
	"net/http"
	"testing"
	"vilib-api/internal/dto"
	"vilib-api/internal/handler"
	"vilib-api/internal/repository"
	"vilib-api/internal/service"
	"vilib-api/test/end2end"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stephenafamo/bob"
	"github.com/stretchr/testify/assert"
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
	end2end.WithDB(t, migrationsPath, func(pool *pgxpool.Pool) {
		sqlDB := stdlib.OpenDBFromPool(pool)
		db := bob.NewDB(sqlDB)
		r := repository.NewTransactionalRepository(&db)
		s := service.NewService(r)
		h := handler.NewHandler(service.NewSagaRunner(s, r), localMailBox)
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

			assert.Equal(t, http.StatusCreated, code)
		})

		adminPassword := <-localMailBox

		assert.NotEmpty(t, adminPassword, "admin password is not created")
	})
}
