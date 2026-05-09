// Package dbmigrate applies golang-migrate migrations on application startup
// when RUN_MIGRATIONS_ON_BOOT=true. Idempotent across deploys (no-op when no
// pending versions); safe under multi-replica boot via PostgreSQL advisory
// locks held by the migrate driver.
package dbmigrate

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"
)

// Run applies all pending migrations from sourceURL against databaseURL.
// sourceURL must be a golang-migrate source URL (e.g. "file:///app/migrations").
// databaseURL must be a Postgres URL accepted by the postgres driver.
// Returns nil on success or when there are no pending migrations.
func Run(sourceURL, databaseURL string, logger *zap.Logger) error {
	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		return fmt.Errorf("dbmigrate: open: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			logger.Warn("dbmigrate: source close error", zap.Error(srcErr))
		}
		if dbErr != nil {
			logger.Warn("dbmigrate: database close error", zap.Error(dbErr))
		}
	}()

	beforeVer, beforeDirty, verErr := m.Version()
	if verErr != nil && !errors.Is(verErr, migrate.ErrNilVersion) {
		return fmt.Errorf("dbmigrate: read current version: %w", verErr)
	}
	if errors.Is(verErr, migrate.ErrNilVersion) {
		logger.Info("dbmigrate: starting from empty schema")
	} else {
		logger.Info("dbmigrate: current schema version",
			zap.Uint("version", uint(beforeVer)), zap.Bool("dirty", beforeDirty))
		if beforeDirty {
			return fmt.Errorf("dbmigrate: refusing to migrate dirty schema at version %d (manual intervention required: migrate force <v>)", beforeVer)
		}
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("dbmigrate: up: %w", err)
	}

	afterVer, afterDirty, _ := m.Version()
	logger.Info("dbmigrate: complete",
		zap.Uint("version", uint(afterVer)), zap.Bool("dirty", afterDirty))
	return nil
}
