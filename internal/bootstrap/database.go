package bootstrap

import (
	"context"
	"fmt"

	"github.com/EnockYator/go-oauth/internal/infrastructure/config"
	"github.com/EnockYator/go-oauth/internal/infrastructure/database/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func openDatabase(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	db, err := postgres.New(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	return db, nil
}
