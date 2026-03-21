package testutil

import (
	"testing"
	"vilib-api/config"
	"vilib-api/internal/repository"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stephenafamo/bob"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// WithDB поднимает временный экземпляр PostgreSQL в контейнере, применяет миграции
// и передаёт инициализированное подключение bob.DB в тестовую функцию.
// Используется для интеграционных тестов с реальной базой данных.
func WithDB(t *testing.T, migrationsPath []string, fn func(bobDB *bob.DB)) {
	dbName := "app"
	dbUser := "user"
	dbPassword := "pass"
	schemaName := "app"
	schemaMigrationsName := "public"

	postgresContainer, err := postgres.Run(t.Context(),
		"postgres:17",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		postgres.BasicWaitStrategies(),
		postgres.WithSQLDriver("pgx"),
	)
	defer func() {
		if err = testcontainers.TerminateContainer(postgresContainer); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()
	if err != nil {
		t.Fatalf("failed to start container: %s", err)
	}

	host, _ := postgresContainer.Host(t.Context())
	port, _ := postgresContainer.MappedPort(t.Context(), "5432")

	dbConfig := config.DatabaseConfig{
		Host:     host,
		Port:     port.Port(),
		User:     dbUser,
		Password: dbPassword,
		Name:     dbName,
		Path:     schemaName,
	}

	dbConfigMigrations := dbConfig
	dbConfigMigrations.Path = schemaMigrationsName

	ctx := t.Context()
	_, stdlibDBConn, err := repository.NewPostgresDB(ctx, dbConfigMigrations)
	if err != nil {
		t.Fatalf("failed to connect to postgres for migrations: %v", err)
	}
	defer stdlibDBConn.Close()

	if err = stdlibDBConn.Ping(ctx); err != nil {
		t.Fatalf("failed to ping postgres for migrations: %v", err)
	}

	stdlibDB := stdlib.OpenDBFromPool(stdlibDBConn)

	for _, path := range migrationsPath {
		if err := goose.Up(stdlibDB, path); err != nil {
			t.Fatalf("failed to apply migrations: %v", err)
		}
	}

	bobDB, dbConn, err := repository.NewPostgresDB(ctx, dbConfig)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	defer dbConn.Close()

	if err = dbConn.Ping(ctx); err != nil {
		t.Fatalf("failed to ping postgres: %v", err)
	}

	fn(bobDB)
}
