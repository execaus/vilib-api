package repository

import (
	"context"
	"fmt"
	"horsy_api/config"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stephenafamo/bob"
)

func NewPostgresDB(cfg *config.DatabaseConfig) (*bob.DB, *pgxpool.Pool, error) {
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?search_path=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Name,
		cfg.Path,
	)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, nil, err
	}

	s := stdlib.OpenDBFromPool(pool)

	db := bob.NewDB(s)

	return &db, pool, nil
}
