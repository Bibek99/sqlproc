package sqlproc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestLoadSchemaMigrations(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "001_init.sql")
	second := filepath.Join(dir, "002_add_idx.sql")
	if err := os.WriteFile(first, []byte("CREATE TABLE test(id INT);"), 0o644); err != nil {
		t.Fatalf("write first: %v", err)
	}
	if err := os.WriteFile(second, []byte("CREATE INDEX idx ON test(id);"), 0o644); err != nil {
		t.Fatalf("write second: %v", err)
	}

	migs, err := LoadSchemaMigrations([]string{second, first})
	if err != nil {
		t.Fatalf("LoadSchemaMigrations error: %v", err)
	}
	if len(migs) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(migs))
	}
	if migs[0].Version != 1 || migs[1].Version != 2 {
		t.Fatalf("migrations not sorted by version: %+v", migs)
	}
}

func TestSchemaMigratorMigrate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	migs := []*SchemaMigration{
		{Version: 1, Name: "init", SQL: "CREATE TABLE test(id INT);"},
		{Version: 2, Name: "add_idx", SQL: "CREATE INDEX idx ON test(id);"},
	}

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS sqlproc_schema_migrations").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT version FROM sqlproc_schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(int64(1)))
	mock.ExpectBegin()
	mock.ExpectExec("CREATE INDEX idx ON test\\(id\\);").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO sqlproc_schema_migrations").
		WithArgs(int64(2), "add_idx").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	migrator := NewSchemaMigrator(db)
	if err := migrator.Migrate(context.Background(), migs); err != nil {
		t.Fatalf("SchemaMigrator.Migrate error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
