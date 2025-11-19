package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Bibek99/sqlproc"
	_ "github.com/lib/pq"
)

func main() {
	var (
		dbURL         = flag.String("db", "", "Database connection string (postgres)")
		filesArg      = flag.String("files", "", "Comma-separated list of SQL files or directories")
		migrationsArg = flag.String("migrations", "", "Comma-separated list of schema migration files or directories")
		outputDir     = flag.String("out", "./generated", "Directory for generated Go package")
		packageName   = flag.String("pkg", "generated", "Go package name for generated code")
		skipMigrate   = flag.Bool("skip-migrate", false, "Skip database migration step")
		skipGenerate  = flag.Bool("skip-generate", false, "Skip code generation")
	)
	flag.Parse()

	if *filesArg == "" {
		log.Fatal("missing -files argument")
	}

	inputs := splitInputs(*filesArg)
	files, err := sqlproc.ResolveFiles(inputs)
	if err != nil {
		log.Fatalf("resolve files: %v", err)
	}

	log.Printf("Found %d SQL file(s)\n", len(files))

	var schemaMigrations []*sqlproc.SchemaMigration
	if *migrationsArg != "" {
		migrationInputs := splitInputs(*migrationsArg)
		migrationFiles, err := sqlproc.ResolveFiles(migrationInputs)
		if err != nil {
			log.Fatalf("resolve migrations: %v", err)
		}
		schemaMigrations, err = sqlproc.LoadSchemaMigrations(migrationFiles)
		if err != nil {
			log.Fatalf("load migrations: %v", err)
		}
		log.Printf("Found %d schema migration(s)\n", len(schemaMigrations))
	}

	var procs []*sqlproc.Procedure
	parser := sqlproc.NewParser()
	if procs, err = parser.ParseFiles(files); err != nil {
		log.Fatalf("parse sql: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if !*skipMigrate {
		if *dbURL == "" {
			log.Fatal("-db is required unless -skip-migrate is set")
		}
		if err := migrate(ctx, *dbURL, schemaMigrations, procs); err != nil {
			log.Fatalf("migration failed: %v", err)
		}
		log.Println("✅ Migration complete")
	}

	if !*skipGenerate {
		gen := sqlproc.NewGenerator(sqlproc.GeneratorOptions{PackageName: *packageName})
		if err := gen.Generate(procs, *outputDir); err != nil {
			log.Fatalf("code generation failed: %v", err)
		}
		log.Printf("✅ Code generated into %s\n", *outputDir)
	}
}

func splitInputs(input string) []string {
	parts := strings.Split(input, ",")
	var cleaned []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return cleaned
}

func migrate(ctx context.Context, url string, schemaMigrations []*sqlproc.SchemaMigration, procs []*sqlproc.Procedure) error {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}
	if len(schemaMigrations) > 0 {
		if err := sqlproc.NewSchemaMigrator(db).Migrate(ctx, schemaMigrations); err != nil {
			return fmt.Errorf("schema migrations: %w", err)
		}
	}
	migrator := sqlproc.NewMigrator(db)
	return migrator.Migrate(ctx, procs)
}
