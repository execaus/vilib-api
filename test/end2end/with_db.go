package end2end

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func WithDB(t *testing.T, migrationsPath []string, fn func(pool *pgxpool.Pool)) {
	dbName := "app"
	dbUser := "user"
	dbPassword := "pass"

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

	dsn := fmt.Sprintf("postgres://user:pass@%s:%s/app?sslmode=disable", host, port.Port())

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err.Error())
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("failed to close sqlDB: %v", err)
		}
	}()

	for _, path := range migrationsPath {
		if err := goose.Up(sqlDB, path); err != nil {
			t.Fatalf("failed to apply migrations: %v", err)
		}
	}

	dbConn, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}

	if err = dbConn.Ping(t.Context()); err != nil {
		t.Fatalf("failed to ping postgres: %v", err)
	}

	fn(dbConn)
}
