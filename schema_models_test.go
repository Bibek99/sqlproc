package sqlproc

import (
	"os"
	"strings"
	"testing"
)

func TestSchemaModelGenerator_WritesStructs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tables := []*Table{
		{
			Schema: "public",
			Name:   "users",
			Columns: []TableColumn{
				{Name: "id", DBType: "int4", Nullable: false},
				{Name: "email", DBType: "text", Nullable: true},
				{Name: "created_at", DBType: "timestamptz", Nullable: false},
			},
		},
	}

	gen := &SchemaModelGenerator{
		Options: SchemaModelOptions{
			OutputDir:   dir,
			PackageName: "models",
			StructTag:   "db,json",
		},
	}
	files, err := gen.Generate(tables)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	content, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	src := string(content)
	if !strings.Contains(src, "type Users struct") {
		t.Fatalf("expected Users struct in output: %s", src)
	}
	if !strings.Contains(src, "`db:\"id\" json:\"id\"`") {
		t.Fatalf("expected struct tags in output: %s", src)
	}
	if !strings.Contains(src, "*string") {
		t.Fatalf("expected nullable column to generate pointer type: %s", src)
	}
	if !strings.Contains(src, "`db:\"created_at\" json:\"createdAt\"`") {
		t.Fatalf("expected camelCase json tag: %s", src)
	}
}

func TestBuildStructTag(t *testing.T) {
	tag := buildStructTag([]string{"db", "json"}, "foo_bar")
	want := "`db:\"foo_bar\" json:\"fooBar\"`"
	if tag != want {
		t.Fatalf("unexpected tag: got %s want %s", tag, want)
	}
}

func TestGoTypeForColumn(t *testing.T) {
	cases := []struct {
		col  TableColumn
		want string
	}{
		{TableColumn{Name: "id", DBType: "int4", Nullable: false}, "int32"},
		{TableColumn{Name: "payload", DBType: "jsonb", Nullable: true}, "[]byte"},
		{TableColumn{Name: "published_at", DBType: "timestamp", Nullable: true}, "*time.Time"},
	}
	for _, c := range cases {
		if got := goTypeForColumn(c.col); got != c.want {
			t.Fatalf("goTypeForColumn(%v) = %s, want %s", c.col, got, c.want)
		}
	}
}
