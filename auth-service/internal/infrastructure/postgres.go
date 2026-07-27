package infrastructure

import (
	"context"
	"fmt"
	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/config"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPostgres(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(buildPostgresDSN(cfg.Postgres))
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}
	poolConfig.MaxConns = cfg.Postgres.MaxConns
	poolConfig.MinConns = cfg.Postgres.MinConns
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
}

func buildPostgresDSN(cfg config.PostgresConfig) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.Username, cfg.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:   cfg.DBName,
	}

	q := u.Query()
	q.Set("sslmode", cfg.SSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}
