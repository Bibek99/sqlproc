package sqlproc

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

// Migrator executes stored procedure definitions against a database.
type Migrator struct {
	db *sql.DB
}

// NewMigrator constructs a migrator.
func NewMigrator(db *sql.DB) *Migrator {
	return &Migrator{db: db}
}

// Migrate executes each procedure on the database.
func (m *Migrator) Migrate(ctx context.Context, procedures []*Procedure) error {
	for _, proc := range procedures {
		if err := m.exec(ctx, proc.SQL); err != nil {
			return fmt.Errorf("migrate %s: %w", proc.File, err)
		}
	}
	return nil
}

// MigrateFiles parses provided SQL files then migrates them.
func (m *Migrator) MigrateFiles(ctx context.Context, files []string) error {
	parser := NewParser()
	procs, err := parser.ParseFiles(files)
	if err != nil {
		return err
	}
	return m.Migrate(ctx, procs)
}

func (m *Migrator) exec(ctx context.Context, sqlText string) error {
	if sqlText == "" {
		return nil
	}
	_, err := m.db.ExecContext(ctx, sqlText)
	return err
}

// GeneratorOptions configure code generation.
type GeneratorOptions struct {
	PackageName string
}

// Generator writes strongly typed Go helpers for stored procedures.
type Generator struct {
	opts GeneratorOptions
}

// NewGenerator constructs a generator.
func NewGenerator(opts GeneratorOptions) *Generator {
	if opts.PackageName == "" {
		opts.PackageName = "generated"
	}
	return &Generator{opts: opts}
}

// GenerateFiles parses SQL files and writes code to outputDir.
func (g *Generator) GenerateFiles(files []string, outputDir string) error {
	parser := NewParser()
	procs, err := parser.ParseFiles(files)
	if err != nil {
		return err
	}
	return g.Generate(procs, outputDir)
}

// Generate writes code for provided procedures.
func (g *Generator) Generate(procs []*Procedure, outputDir string) error {
	if len(procs) == 0 {
		return fmt.Errorf("no procedures to generate")
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	cg := &CodeGenerator{
		OutputDir:   outputDir,
		PackageName: g.opts.PackageName,
	}
	return cg.Generate(procs)
}

// GenerateToTemp parses files and writes code to a temporary directory, returning the path.
func (g *Generator) GenerateToTemp(files []string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "sqlproc-gen-*")
	if err != nil {
		return "", err
	}
	if err := g.GenerateFiles(files, tmpDir); err != nil {
		return "", err
	}
	return tmpDir, nil
}

// ResolveFiles expands mixed directories/file inputs into a list of SQL files.
func ResolveFiles(inputs []string) ([]string, error) {
	var files []string
	for _, in := range inputs {
		if in == "" {
			continue
		}
		info, err := os.Stat(in)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", in, err)
		}
		if info.IsDir() {
			dirFiles, err := CollectSQLFiles(in)
			if err != nil {
				return nil, err
			}
			files = append(files, dirFiles...)
			continue
		}
		if filepath.Ext(in) != ".sql" {
			continue
		}
		files = append(files, in)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no SQL files resolved from %v", inputs)
	}
	return files, nil
}
