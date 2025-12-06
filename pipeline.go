package sqlproc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Logger is a minimal logging interface used by the orchestration pipeline.
type Logger interface {
	Printf(format string, v ...any)
}

// PipelineOptions configure orchestration of migrations and code generation.
type PipelineOptions struct {
	// SQLInputs are directories and/or files containing stored procedure SQL.
	SQLInputs []string
	// MigrationInputs are directories/files with schema migrations. Optional.
	MigrationInputs []string
	// OutputDir is where generated Go files are written. Defaults to ./generated.
	OutputDir string
	// PackageName overrides the generated Go package name. Defaults to "generated".
	PackageName string
	// SkipMigrate toggles executing migrations against a database.
	SkipMigrate bool
	// SkipGenerate toggles emitting Go code.
	SkipGenerate bool
	// DB allows callers to supply an existing *sql.DB handle.
	DB *sql.DB
	// DBURL is used to lazily open a connection when DB is nil.
	DBURL string
	// DBDriver is the sql driver name used with DBURL (default: "postgres").
	DBDriver string
	// Parser allows providing a custom parser. Defaults to NewParser().
	Parser *Parser
	// GeneratorOptions are passed to the Go code generator.
	GeneratorOptions GeneratorOptions
	// Logger emits progress logs. Defaults to a standard logger writing to stdout.
	Logger Logger
	// SchemaModels controls schema-introspection-based model generation.
	SchemaModels *SchemaModelOptions
}

// PipelineResult captures the work performed by Run.
type PipelineResult struct {
	Procedures       []*Procedure
	SchemaMigrations []*SchemaMigration
	OutputDir        string
	GeneratedFiles   []string
	SchemaTables     []*Table
	SchemaFiles      []string
}

// Run executes the configured pipeline: resolve -> parse -> migrate -> generate.
func Run(ctx context.Context, opts PipelineOptions) (*PipelineResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(opts.SQLInputs) == 0 && len(opts.MigrationInputs) == 0 && opts.SchemaModels == nil {
		return nil, errors.New("sqlproc: provide SQL inputs, migrations, or schema model options")
	}

	logWriter := opts.Logger
	if logWriter == nil {
		logWriter = log.New(os.Stdout, "[sqlproc] ", log.LstdFlags)
	}

	parser := opts.Parser
	if parser == nil {
		parser = NewParser()
	}

	var procs []*Procedure
	if len(opts.SQLInputs) > 0 {
		sqlFiles, err := ResolveFiles(opts.SQLInputs)
		if err != nil {
			return nil, fmt.Errorf("resolve SQL inputs: %w", err)
		}
		logWriter.Printf("resolved %d SQL file(s)", len(sqlFiles))

		procs, err = parser.ParseFiles(sqlFiles)
		if err != nil {
			return nil, fmt.Errorf("parse SQL files: %w", err)
		}
	} else {
		logWriter.Printf("no stored procedure inputs provided; skipping procedure parsing")
	}

	var schemaMigrations []*SchemaMigration
	if len(opts.MigrationInputs) > 0 {
		migFiles, err := ResolveFiles(opts.MigrationInputs)
		if err != nil {
			return nil, fmt.Errorf("resolve migration inputs: %w", err)
		}
		schemaMigrations, err = LoadSchemaMigrations(migFiles)
		if err != nil {
			return nil, fmt.Errorf("load migrations: %w", err)
		}
		logWriter.Printf("resolved %d schema migration(s)", len(schemaMigrations))
	}

	db, cleanup, err := prepareDB(ctx, opts)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	if !opts.SkipMigrate {
		if db == nil {
			return nil, errors.New("sqlproc: DB or DBURL must be provided when migrations are enabled")
		}
		if len(schemaMigrations) > 0 {
			logWriter.Printf("applying %d schema migration(s)", len(schemaMigrations))
		}
		if len(procs) > 0 {
			logWriter.Printf("applying %d stored procedure(s)", len(procs))
		}
		if err := runMigrations(ctx, db, schemaMigrations, procs); err != nil {
			return nil, err
		}
	}

	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = "./generated"
	}

	defaultPackage := opts.PackageName
	if defaultPackage == "" {
		defaultPackage = opts.GeneratorOptions.PackageName
	}
	if defaultPackage == "" {
		defaultPackage = "generated"
	}

	var generatedFiles []string
	if !opts.SkipGenerate {
		pkgName := opts.PackageName
		if pkgName == "" {
			pkgName = opts.GeneratorOptions.PackageName
		}
		if pkgName == "" {
			pkgName = "generated"
		}
		if len(procs) == 0 {
			logWriter.Printf("no procedures to generate; skipping code emission")
		} else {
			genOpts := opts.GeneratorOptions
			genOpts.PackageName = pkgName
			gen := NewGenerator(genOpts)
			if err := gen.Generate(procs, outputDir); err != nil {
				return nil, fmt.Errorf("generate Go code: %w", err)
			}
			generatedFiles = []string{
				filepath.Join(outputDir, "db.go"),
				filepath.Join(outputDir, "models.go"),
				filepath.Join(outputDir, "queries.go"),
			}
			logWriter.Printf("generated Go package %q in %s", pkgName, outputDir)
		}
	}

	var schemaTables []*Table
	var schemaFiles []string
	if opts.SchemaModels != nil {
		if db == nil {
			return nil, errors.New("sqlproc: schema model generation requires a database connection or DBURL")
		}
		schemaOpts := opts.SchemaModels.withDefaults(outputDir, defaultPackage)
		var err error
		schemaTables, err = loadSchemaTables(ctx, db, schemaOpts)
		if err != nil {
			return nil, fmt.Errorf("introspect schema: %w", err)
		}
		generator := &SchemaModelGenerator{Options: schemaOpts}
		schemaFiles, err = generator.Generate(schemaTables)
		if err != nil {
			return nil, fmt.Errorf("generate schema models: %w", err)
		}
		if len(schemaTables) > 0 {
			logWriter.Printf("generated %d schema model(s) in %s", len(schemaTables), schemaOpts.OutputDir)
		} else {
			logWriter.Printf("no tables discovered for schema model generation")
		}
	}

	return &PipelineResult{
		Procedures:       procs,
		SchemaMigrations: schemaMigrations,
		OutputDir:        outputDir,
		GeneratedFiles:   generatedFiles,
		SchemaTables:     schemaTables,
		SchemaFiles:      schemaFiles,
	}, nil
}

func prepareDB(ctx context.Context, opts PipelineOptions) (*sql.DB, func(), error) {
	db := opts.DB
	if db != nil {
		return db, nil, nil
	}
	if opts.DBURL == "" {
		return nil, nil, nil
	}
	driver := opts.DBDriver
	if driver == "" {
		driver = "postgres"
	}
	newDB, err := sql.Open(driver, opts.DBURL)
	if err != nil {
		return nil, nil, fmt.Errorf("open db: %w", err)
	}
	if err := newDB.PingContext(ctx); err != nil {
		_ = newDB.Close()
		return nil, nil, fmt.Errorf("ping db: %w", err)
	}
	return newDB, func() { _ = newDB.Close() }, nil
}

func runMigrations(ctx context.Context, db *sql.DB, schemaMigrations []*SchemaMigration, procs []*Procedure) error {
	if len(schemaMigrations) > 0 {
		if err := NewSchemaMigrator(db).Migrate(ctx, schemaMigrations); err != nil {
			return fmt.Errorf("schema migrations: %w", err)
		}
	}
	if err := NewMigrator(db).Migrate(ctx, procs); err != nil {
		return fmt.Errorf("procedure migrations: %w", err)
	}
	return nil
}
