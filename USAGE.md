# Usage Guide

## 1. Write SQL procedures

Store each procedure/function in its own `.sql` file and add metadata comments:

```
-- name: CreateUser :one
-- param: name text
-- param: email text
-- returns: id int, name text, email text, created_at timestamp
```

Supported return markers:

- `:one` – function returns a single row
- `:many` – function returns multiple rows
- `:exec` – function returns nothing (side-effects only)

## 2. Run the CLI

```
sqlproc \
  -db "postgres://postgres:postgres@localhost:5432/sqlproc?sslmode=disable" \
  -files ./path/to/sql \
  -out ./generated \
  -pkg generated
```

Flags:

| Flag | Description |
| ---- | ----------- |
| `-db` | PostgreSQL connection string (omit with `-skip-migrate`) |
| `-files` | Comma-separated paths (files or directories) containing SQL |
| `-out` | Output folder for generated Go code |
| `-pkg` | Package name to use inside generated files |
| `-skip-migrate` | Only generate code, do not execute SQL |
| `-skip-generate` | Only run migrations, do not emit Go code |

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

1. Auto-running migrations on start-up
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

