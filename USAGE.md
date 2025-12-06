# Usage Guide

## 1. Write SQL procedures

Store each procedure/function in its own `.sql` file and add metadata comments:

```
-- name: CreateUser :one
-- param: name text
-- param: email text
-- returns: id int, name text, email text, created_at timestamptz
```

Supported return markers:

- `:one` – function returns a single row
- `:many` – function returns multiple rows
- `:exec` – function returns nothing (side-effects only)

## 2. Run the CLI

```
sqlproc \
  -db "postgres://postgres:postgres@localhost:5432/sqlproc?sslmode=disable" \
  -migrations ./path/to/schema/migrations \
  -files ./path/to/sql \
  -out ./generated \
  -pkg generated
```

Schema migration files are plain SQL statements named with an increasing numeric prefix such as `001_init.sql`, `002_add_index.sql`. They run once and are tracked inside the `sqlproc_schema_migrations` table.

Flags:

| Flag | Description |
| ---- | ----------- |
| `-db` | PostgreSQL connection string (omit with `-skip-migrate`) |
| `-files` | Comma-separated paths (files or directories) containing SQL |
| `-migrations` | Comma-separated paths containing schema migration SQL |
| `-out` | Output folder for generated Go code |
| `-pkg` | Package name to use inside generated files |
| `-skip-migrate` | Only generate code, do not execute SQL |
| `-skip-generate` | Only run migrations, do not emit Go code |
| `-schema-models` | Introspect tables and emit Go structs after migrations |
| `-schema-out` | Output directory for schema structs (default `-out`) |
| `-schema-pkg` | Package name for schema structs (default `-pkg`) |
| `-schemas` | Schemas to introspect (comma-separated, `*` = all user schemas) |
| `-schema-tag` | Struct tag keys (comma-separated, default `db,json`) |

## 2b. Embed inside your Go service

You can run the exact same pipeline from Go by importing the module:

```go
ctx := context.Background()
db, _ := sql.Open("postgres", os.Getenv("DATABASE_URL"))

_, err := sqlproc.Run(ctx, sqlproc.PipelineOptions{
	SQLInputs:       []string{"./db/funcs"},
	MigrationInputs: []string{"./db/migrations"},
	OutputDir:       "./internal/db",
	PackageName:     "db",
	DB:              db,
})
if err != nil {
	log.Fatalf("bootstrap db: %v", err)
}
```

Supplying `DBURL` instead of `DB` works as long as the driver (e.g. `_ "github.com/lib/pq"`) is imported.

## 2c. Generate models directly from schema migrations

Enable schema introspection to keep your Go structs synchronized with table DDL, even when no stored procedure files are present:

```bash
sqlproc \
  -db "$DATABASE_URL" \
  -migrations ./db/migrations \
  -schema-models \
  -schema-out ./internal/db \
  -schema-pkg db \
  -schemas public \
  -schema-tag "db,json"
```

Or from Go:

```go
_, err := sqlproc.Run(ctx, sqlproc.PipelineOptions{
	MigrationInputs: []string{"./db/migrations"},
	DB:              db,
	SkipGenerate:    true,
	SchemaModels: &sqlproc.SchemaModelOptions{
		OutputDir:   "./internal/db",
		PackageName: "db",
		Schemas:     []string{"public"},
		StructTag:   "db,json",
	},
})
```

## 3. Use the generated package

```go
import "your/module/generated"

db, _ := sql.Open("postgres", dsn)
queries := generated.New(db)

user, err := queries.CreateUser(ctx, "Jane", "jane@example.com")
list, err := queries.ListUsers(ctx)
err := queries.DeleteUser(ctx, user.Id)
```

## 4. Backend example

The sample backend (`examples/backend`) demonstrates:

1. Auto-running schema migrations and stored procedures on start-up
2. Serving REST endpoints on top of generated code
3. Simple JSON handlers using standard `net/http`

Run it with:

```
go run ./examples/backend
```

Environment variables:

| Variable | Default | Purpose |
| -------- | ------- | ------- |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/sqlproc?sslmode=disable` | DB connection |
| `ADDR` | `:8080` | HTTP bind address |

