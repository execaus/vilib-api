package service_test

import (
	"strings"
	"testing"
	"vilib-api/config"
	"vilib-api/internal/repository"
	"vilib-api/internal/service"
	"vilib-api/server"
	"vilib-api/testutil"

	"github.com/google/uuid"
	"github.com/stephenafamo/bob"
	"github.com/stretchr/testify/require"
)

// Проверяет на настоящей PostgreSQL, что название организации не уникально (A-01 ТЗ): две
// организации с одинаковым названием и одинаковой локальной частью email владельца
// регистрируются независимо, а название берётся из запроса, а не из адреса.
func TestService_AccountCreate_SameNameIsAllowed(t *testing.T) {
	t.Parallel()

	testutil.WithDB(t, []string{"../../migrations"}, func(bobDB *bob.DB) {
		repo := repository.NewRepository(repository.NewExecutorProvider(bobDB))
		cfg := config.Config{Server: config.ServerConfig{Mode: server.DevelopmentMode}}
		srv := service.NewService(cfg, nil, nil, repo)

		const accountName = "ООО «Ромашка»"

		local := strings.ToLower(testutil.Faker.Person().FirstName()) + "-" + uuid.NewString()[:8]
		surname := testutil.Faker.Person().LastName()

		first, err := srv.Account.Create(
			t.Context(), accountName, testutil.Faker.Person().FirstName(), surname, local+"@first.test",
		)
		require.NoError(t, err)

		second, err := srv.Account.Create(
			t.Context(), "  "+accountName+"  ", testutil.Faker.Person().FirstName(), surname, local+"@second.test",
		)
		require.NoError(t, err)

		require.NotEqual(t, first.ID, second.ID)
		require.Equal(t, accountName, first.Name)
		require.Equal(t, accountName, second.Name)
	})
}
