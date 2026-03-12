package repository

import (
	"context"
	"fmt"
	"net"
	"vilib-api/config"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stephenafamo/bob"
)

func NewPostgresDB(cfg *config.DatabaseConfig) (*bob.DB, *pgxpool.Pool, error) {
	hostPort := net.JoinHostPort(cfg.Host, cfg.Port)

	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s/%s?search_path=%s",
		cfg.User,
		cfg.Password,
		hostPort,
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
