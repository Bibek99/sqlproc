package sqlproc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRun_GenerateOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sqlFile := writeTestFile(t, dir, "ping.sql", sampleProcedureSQL())

	outDir := filepath.Join(dir, "generated")
	result, err := Run(context.Background(), PipelineOptions{
		SQLInputs:    []string{sqlFile},
		OutputDir:    outDir,
		SkipMigrate:  true,
		PackageName:  "autogen",
		SkipGenerate: false,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(result.GeneratedFiles) != 3 {
		t.Fatalf("expected 3 generated files, got %d", len(result.GeneratedFiles))
	}
	for _, file := range result.GeneratedFiles {
		if _, err := os.Stat(file); err != nil {
			t.Fatalf("expected generated file %s to exist: %v", file, err)
		}
	}
}

func TestRun_MissingDB(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sqlFile := writeTestFile(t, dir, "ping.sql", sampleProcedureSQL())

	_, err := Run(context.Background(), PipelineOptions{
		SQLInputs: []string{sqlFile},
	})
	if err == nil {
		t.Fatal("expected error when DB is missing and migrations are enabled")
	}
}

func TestRun_WithDBAndMigrations(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sqlFile := writeTestFile(t, dir, "ping.sql", sampleProcedureSQL())
	migrationFile := writeTestFile(t, dir, "001_init.sql", "CREATE TABLE foo(id INT PRIMARY KEY);")

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS sqlproc_schema_migrations`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT version FROM sqlproc_schema_migrations`).
		WillReturnRows(sqlmock.NewRows([]string{"version"}))
	mock.ExpectBegin()
	mock.ExpectExec(`CREATE TABLE foo`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`INSERT INTO sqlproc_schema_migrations`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec(`CREATE OR REPLACE FUNCTION ping_proc`).WillReturnResult(sqlmock.NewResult(0, 0))

	_, err = Run(context.Background(), PipelineOptions{
		SQLInputs:       []string{sqlFile},
		MigrationInputs: []string{migrationFile},
		DB:              db,
		SkipGenerate:    true,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRun_SchemaModelsOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	queryRegex := "SELECT table_schema, table_name, column_name, data_type, udt_name, is_nullable"
	rows := sqlmock.NewRows([]string{"table_schema", "table_name", "column_name", "data_type", "udt_name", "is_nullable"}).
		AddRow("public", "users", "id", "integer", "int4", "NO").
		AddRow("public", "users", "email", "text", "text", "YES")
	mock.ExpectQuery(queryRegex).WithArgs("public").WillReturnRows(rows)

	result, err := Run(context.Background(), PipelineOptions{
		SkipMigrate:  true,
		DB:           db,
		SchemaModels: &SchemaModelOptions{Schemas: []string{"public"}, OutputDir: dir, PackageName: "models"},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(result.SchemaFiles) != 1 {
		t.Fatalf("expected 1 schema model file, got %d", len(result.SchemaFiles))
	}
	if _, err := os.Stat(result.SchemaFiles[0]); err != nil {
		t.Fatalf("expected schema file to exist: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRun_SchemaModelsRequireDB(t *testing.T) {
	t.Parallel()
	_, err := Run(context.Background(), PipelineOptions{
		SchemaModels: &SchemaModelOptions{Schemas: []string{"public"}, OutputDir: "./tmp"},
	})
	if err == nil {
		t.Fatal("expected error when schema models requested without DB")
	}
}

func writeTestFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
	return path
}

func sampleProcedureSQL() string {
	return `-- name: Ping :exec
CREATE OR REPLACE FUNCTION ping_proc()
RETURNS void AS $$
BEGIN
	PERFORM 1;
END;
$$ LANGUAGE plpgsql;`
}
