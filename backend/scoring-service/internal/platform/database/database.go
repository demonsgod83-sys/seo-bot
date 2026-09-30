package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"go.uber.org/zap"
)

type SQLDatabase struct {
	db     *bun.DB
	logger *zap.Logger
}

func NewSQLDatabase(logger *zap.Logger) *SQLDatabase {
	return &SQLDatabase{logger: logger}
}

func (s *SQLDatabase) Connect(ctx context.Context, dsn string) error {
	sqlDB := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	db := bun.NewDB(sqlDB, pgdialect.New())

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	s.db = db
	s.logger.Info("database connected")
	return nil
}

func (s *SQLDatabase) RegisterModels(ctx context.Context, models ...interface{}) error {
	s.db.RegisterModel(models...)
	for _, model := range models {
		if _, err := s.db.NewCreateTable().Model(model).IfNotExists().Exec(ctx); err != nil {
			return fmt.Errorf("error creating table for model: %w", err)
		}
	}
	return nil
}

func (s *SQLDatabase) GetDBClient() *bun.DB {
	return s.db
}
