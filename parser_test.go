package sqlproc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParserParseFile(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "proc.sql")
	sql := `-- name: GetUser :one
-- param: user_id int
-- returns: id int, name text

CREATE OR REPLACE FUNCTION get_user(p_user_id INT)
RETURNS TABLE(id INT, name TEXT) AS $$
BEGIN
    RETURN QUERY SELECT id, name FROM users WHERE id = p_user_id;
END;
$$ LANGUAGE plpgsql;
`
	if err := os.WriteFile(file, []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}

	parser := NewParser()
	proc, err := parser.ParseFile(file)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if proc.Name != "GetUser" {
		t.Fatalf("expected GetUser, got %s", proc.Name)
	}
	if proc.Kind != ReturnOne {
		t.Fatalf("expected ReturnOne, got %s", proc.Kind)
	}
	if len(proc.Params) != 1 || proc.Params[0].Name != "user_id" {
		t.Fatalf("unexpected params: %+v", proc.Params)
	}
	if len(proc.Returns) != 2 {
		t.Fatalf("expected 2 return columns, got %d", len(proc.Returns))
	}
	if proc.SQL == "" {
		t.Fatal("SQL should not be empty")
	}
}

func TestResolveFiles(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "sql")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := []string{
		filepath.Join(dir, "a.sql"),
		filepath.Join(dir, "b.sql"),
	}
	for _, f := range files {
		if err := os.WriteFile(f, []byte("-- name: A :exec\nSELECT 1;"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	resolved, err := ResolveFiles([]string{dir})
	if err != nil {
		t.Fatalf("ResolveFiles error: %v", err)
	}
	if len(resolved) != len(files) {
		t.Fatalf("expected %d files, got %d", len(files), len(resolved))
	}
}

