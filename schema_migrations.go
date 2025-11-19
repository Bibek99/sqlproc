package sqlproc

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const schemaMigrationsTable = "sqlproc_schema_migrations"

var migrationFilenamePattern = regexp.MustCompile(`^(\d+)[-_]?([A-Za-z0-9_-]*)\.sql$`)

// SchemaMigration represents a discrete schema change.
type SchemaMigration struct {
	Version int64
	Name    string
	File    string
	SQL     string
}

// LoadSchemaMigrations reads raw SQL migration files and returns structured migrations.
func LoadSchemaMigrations(files []string) ([]*SchemaMigration, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("no schema migration files provided")
	}
	migrations := make([]*SchemaMigration, 0, len(files))
	seen := make(map[int64]string)
	for _, file := range files {
		mig, err := parseSchemaMigration(file)
		if err != nil {
			return nil, err
		}
		if existing, ok := seen[mig.Version]; ok {
			return nil, fmt.Errorf("duplicate schema migration version %d (%s and %s)", mig.Version, existing, file)
		}
		seen[mig.Version] = file
		migrations = append(migrations, mig)
	}
	sort.Slice(migrations, func(i, j int) bool {
		if migrations[i].Version == migrations[j].Version {
			return migrations[i].Name < migrations[j].Name
		}
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}

func parseSchemaMigration(path string) (*SchemaMigration, error) {
	base := filepath.Base(path)
	matches := migrationFilenamePattern.FindStringSubmatch(base)
	if matches == nil {
		return nil, fmt.Errorf("invalid migration filename %q (expected NN_description.sql)", base)
	}
	version, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse migration version from %q: %w", base, err)
	}
	if version <= 0 {
		return nil, fmt.Errorf("migration version must be positive in %q", base)
	}
	name := strings.Trim(matches[2], "-_ ")
	if name == "" {
		name = fmt.Sprintf("migration_%d", version)
	}
	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read migration %s: %w", path, err)
	}
	sqlText := strings.TrimSpace(string(sqlBytes))
	if sqlText == "" {
		return nil, fmt.Errorf("migration %s is empty", path)
	}
	return &SchemaMigration{
		Version: version,
		Name:    name,
		File:    path,
		SQL:     sqlText,
	}, nil
}

// SchemaMigrator applies schema migrations with version tracking.
type SchemaMigrator struct {
	db *sql.DB
}

// NewSchemaMigrator creates a schema migrator.
func NewSchemaMigrator(db *sql.DB) *SchemaMigrator {
	return &SchemaMigrator{db: db}
}

// Migrate applies all pending schema migrations in order.
func (m *SchemaMigrator) Migrate(ctx context.Context, migrations []*SchemaMigration) error {
	if len(migrations) == 0 {
		return nil
	}
	if err := m.ensureTable(ctx); err != nil {
		return err
	}
	applied, err := m.appliedVersions(ctx)
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		if applied[migration.Version] {
			continue
		}
		if err := m.applyMigration(ctx, migration); err != nil {
			return err
		}
	}
	return nil
}

func (m *SchemaMigrator) ensureTable(ctx context.Context) error {
	const createTable = `
CREATE TABLE IF NOT EXISTS ` + schemaMigrationsTable + ` (
	version BIGINT PRIMARY KEY,
	name TEXT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	_, err := m.db.ExecContext(ctx, createTable)
	return err
}

func (m *SchemaMigrator) appliedVersions(ctx context.Context) (map[int64]bool, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT version FROM `+schemaMigrationsTable)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int64]bool)
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

func (m *SchemaMigrator) applyMigration(ctx context.Context, migration *SchemaMigration) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply migration %s: %w", migration.File, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO `+schemaMigrationsTable+` (version, name) VALUES ($1, $2)`, migration.Version, migration.Name); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
