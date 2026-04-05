package testutil

import (
	"testing"
	"vilib-api/internal/repository"

	"github.com/jaswdr/faker/v2"
	"github.com/stephenafamo/bob"
)

var (
	migrationsPath = []string{"../../migrations"}
)

func TestRepositoryWithDB(t *testing.T, fn func(r *repository.Repository, f faker.Faker)) {
	t.Helper()

	WithDB(t, migrationsPath, func(bobDB *bob.DB) {
		t.Helper()
		fn(repository.NewRepository(repository.NewExecutorProvider(bobDB)), Faker)
	})
}
