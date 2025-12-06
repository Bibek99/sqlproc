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
		schemaModels  = flag.Bool("schema-models", false, "Generate Go structs by introspecting the database schema")
		schemaOut     = flag.String("schema-out", "", "Directory for schema model files (defaults to -out)")
		schemaPkg     = flag.String("schema-pkg", "", "Package name for schema models (defaults to -pkg)")
		schemaList    = flag.String("schemas", "public", "Comma-separated database schemas to introspect (use * for all)")
		schemaTag     = flag.String("schema-tag", "db,json", "Comma-separated struct tag keys (e.g. \"db,json\")")
	)
	flag.Parse()

	if *filesArg == "" && strings.TrimSpace(*migrationsArg) == "" && !*schemaModels {
		log.Fatal("provide -files, -migrations, or enable -schema-models")
	}

	sqlInputs := splitInputs(*filesArg)
	migrationInputs := splitInputs(*migrationsArg)
	if !*skipMigrate && *dbURL == "" {
		log.Fatal("-db is required unless -skip-migrate is set")
	}

	var schemaOpts *sqlproc.SchemaModelOptions
	if *schemaModels {
		schemas := splitInputs(*schemaList)
		if len(schemas) == 0 && strings.TrimSpace(*schemaList) == "" {
			schemas = nil
		}
		if len(schemas) == 1 && schemas[0] == "*" {
			schemas = nil
		}
		tag := strings.TrimSpace(*schemaTag)
		schemaOpts = &sqlproc.SchemaModelOptions{
			Schemas:     schemas,
			OutputDir:   firstNonEmpty(*schemaOut, *outputDir),
			PackageName: firstNonEmpty(*schemaPkg, *packageName),
			StructTag:   tag,
		}
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
		SchemaModels:    schemaOpts,
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
	if len(result.SchemaFiles) > 0 {
		log.Printf("✅ Schema model files: %s", strings.Join(result.SchemaFiles, ", "))
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
