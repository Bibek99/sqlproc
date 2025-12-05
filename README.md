# sqlproc

`sqlproc` is a Go library and CLI that turns stored procedure SQL files into type-safe Go code. Point it at a directory of `.sql` files that include lightweight metadata comments and it will:

- migrate the stored procedures into your database
- optionally run plain SQL schema migrations with version tracking
- generate Go structs for procedure parameters and return rows
- emit type-safe helper functions to execute those procedures and return typed responses
- power a real backend — an example REST API is included in `examples/backend`

## Quick start

```bash
git clone https://github.com/Bibek99/sqlproc.git
cd sqlproc
go install ./cmd/sqlproc

# Run schema + procedure migrations, then generate code
sqlproc \
  -db "postgres://postgres:postgres@localhost:5432/sqlproc?sslmode=disable" \
  -migrations ./examples/backend/migrations \
  -files ./examples/backend/sql \
  -out ./examples/backend/generated \
  -pkg generated
```

## Use it as a Go module

Embed `sqlproc` directly inside your backend to orchestrate migrations and code generation programmatically:

```go
package dbbootstrap

import (
	"context"
	"database/sql"

	"github.com/Bibek99/sqlproc"
)

func Prepare(ctx context.Context, db *sql.DB) error {
	_, err := sqlproc.Run(ctx, sqlproc.PipelineOptions{
		SQLInputs:       []string{"./db/funcs"},
		MigrationInputs: []string{"./db/migrations"},
		OutputDir:       "./internal/db",
		PackageName:     "db",
		DB:              db,
	})
	return err
}
```

Provide either an existing `*sql.DB` (as above) or a `DBURL` + driver name. When `SkipGenerate` is `false`, the Go files are emitted to `OutputDir`, making the package ready for your module.

### SQL metadata format

Each stored procedure/function should include header comments so the parser can infer types:

```sql
-- name: GetUser :one          -- :one | :many | :exec
-- param: user_id int          -- repeat per parameter
-- returns: id int, name text  -- only needed for :one/:many
CREATE OR REPLACE FUNCTION get_user(p_user_id INT)
RETURNS TABLE(id INT, name TEXT) AS $$
BEGIN
  RETURN QUERY SELECT id, name FROM users WHERE id = p_user_id;
END;
$$ LANGUAGE plpgsql;
```

### Schema migrations

For `CREATE TABLE`, `ALTER TABLE`, and other DDL, drop raw SQL files into a directory (for example `migrations/001_init.sql`, `migrations/002_add_index.sql`). Provide that directory via `-migrations` and `sqlproc` will execute each file once, recording applied versions inside `sqlproc_schema_migrations`.

### From SQL to Go

The generated package exposes a `Queries` type with one method per procedure:

```go
db, _ := sql.Open("postgres", dsn)
queries := generated.New(db)
user, err := queries.GetUser(ctx, 42)
list, err := queries.ListUsers(ctx)
err := queries.DeleteUser(ctx, 42)
```

## Example backend

A runnable REST API lives in `examples/backend`. It:

1. Runs schema migrations from `examples/backend/migrations`
2. Migrates the procedures found in `examples/backend/sql`
3. Uses the generated package (`examples/backend/generated`) to serve HTTP routes

Run it after configuring PostgreSQL:

```bash
createdb sqlproc        # or docker run postgres …
go run ./examples/backend
```

Available endpoints:

- `GET /users` – list users
- `POST /users` – create (body: `{ "name": "...", "email": "..." }`)
- `GET /users/{id}` – fetch single user
- `PUT /users/{id}` – update email
- `DELETE /users/{id}` – remove user

## CLI reference

```
Usage: sqlproc -files <path>[,<path>...] [options]

  -db string
        Database connection string (postgres)
  -files string
        Comma-separated list of SQL files or directories
  -migrations string
        Comma-separated list of schema migration SQL files or directories
  -out string
        Output directory for generated code (default "./generated")
  -pkg string
        Package name for generated code (default "generated")
  -skip-generate
        Skip code generation
  -skip-migrate
        Skip database migration
```

## Development

```bash
go test ./...
go run ./cmd/sqlproc -skip-migrate -files ./examples/backend/sql -out ./examples/backend/generated
go run ./examples/backend
```

## License

MIT
