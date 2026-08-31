package service_test

import (
	"testing"
	"vilib-api/config"
	"vilib-api/internal/repository"
	"vilib-api/internal/service"
	"vilib-api/server"
	"vilib-api/testutil"

	"github.com/stephenafamo/bob"
	"github.com/stretchr/testify/require"
)

// Проверяет распознавание нарушения уникальности имени аккаунта на настоящей PostgreSQL
// (В-11): сентинел dberrors.AccountErrors.ErrUniqueAccountsNameKey реализует Is(target error)
// с target *pgconn.PgError, поэтому [errors.Is](sentinel, err) — единственный рабочий порядок
// аргументов. На моках, где возвращается сам сентинел, ошибка в порядке аргументов не видна.
func TestService_AccountCreate_DuplicateNameReturnsErrAccountNameExists(t *testing.T) {
	t.Parallel()

	testutil.WithDB(t, []string{"../../migrations"}, func(bobDB *bob.DB) {
		repo := repository.NewRepository(repository.NewExecutorProvider(bobDB))
		cfg := config.Config{Server: config.ServerConfig{Mode: server.DevelopmentMode}}
		srv := service.NewService(cfg, nil, nil, repo)

		email := testutil.Faker.Person().Contact().Email
		surname := testutil.Faker.Person().LastName()

		firstName := testutil.Faker.Person().FirstName()
		_, err := srv.Account.Create(t.Context(), firstName, surname, email)
		require.NoError(t, err)

		// Второй аккаунт делит с первым вычисленное из email имя — вставка в БД
		// нарушает уникальность accounts.name и возвращает *pgconn.PgError.
		secondName := testutil.Faker.Person().FirstName()
		_, err = srv.Account.Create(t.Context(), secondName, surname, email)

		require.ErrorIs(t, err, service.ErrAccountNameExists)
	})
}
