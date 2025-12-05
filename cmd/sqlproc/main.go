package main

import (
	"context"
	"flag"
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

	sqlInputs := splitInputs(*filesArg)
	migrationInputs := splitInputs(*migrationsArg)
	if !*skipMigrate && *dbURL == "" {
		log.Fatal("-db is required unless -skip-migrate is set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := sqlproc.Run(ctx, sqlproc.PipelineOptions{
		SQLInputs:       sqlInputs,
		MigrationInputs: migrationInputs,
		OutputDir:       *outputDir,
		PackageName:     *packageName,
		SkipMigrate:     *skipMigrate,
		SkipGenerate:    *skipGenerate,
		DBURL:           *dbURL,
	})
	if err != nil {
		log.Fatalf("sqlproc failed: %v", err)
	}

	log.Printf("✅ Processed %d procedure(s)", len(result.Procedures))
	if len(result.SchemaMigrations) > 0 {
		log.Printf("✅ Applied %d schema migration(s)", len(result.SchemaMigrations))
	}
	if len(result.GeneratedFiles) > 0 {
		log.Printf("✅ Generated files: %s", strings.Join(result.GeneratedFiles, ", "))
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
