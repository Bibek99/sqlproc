package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/bibek/sqlproc"
	"github.com/bibek/sqlproc/examples/backend/generated"
	_ "github.com/lib/pq"
)

func main() {
	dbURL := envOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/sqlproc?sslmode=disable")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	if err := ensureSchema(ctx, db); err != nil {
		log.Fatalf("ensure schema: %v", err)
	}

	if err := migrateProcedures(ctx, db); err != nil {
		log.Fatalf("migrate procedures: %v", err)
	}

	server := &Server{
		db:      db,
		queries: generated.New(db),
	}

	addr := envOrDefault("ADDR", ":8080")
	log.Printf("Backend running on %s", addr)
	if err := http.ListenAndServe(addr, server.routes()); err != nil {
		log.Fatal(err)
	}
}

type Server struct {
	db      *sql.DB
	queries *generated.Queries
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", s.handleUsers)
	mux.HandleFunc("/users/", s.handleUserByID)
	return mux
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listUsers(w, r)
	case http.MethodPost:
		s.createUser(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUserByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/users/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.getUser(w, r, id)
	case http.MethodPut:
		s.updateUser(w, r, id)
	case http.MethodDelete:
		s.deleteUser(w, r, id)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.queries.ListUsers(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, users, http.StatusOK)
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request, id int) {
	user, err := s.queries.GetUser(r.Context(), int32(id))
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, user, http.StatusOK)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	user, err := s.queries.CreateUser(r.Context(), payload.Name, payload.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, user, http.StatusCreated)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request, id int) {
	var payload struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	user, err := s.queries.UpdateUser(r.Context(), int32(id), payload.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, user, http.StatusOK)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request, id int) {
	if err := s.queries.DeleteUser(r.Context(), int32(id)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, v any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func ensureSchema(ctx context.Context, db *sql.DB) error {
	const sqlStmt = `
CREATE TABLE IF NOT EXISTS users (
	id SERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	email TEXT NOT NULL UNIQUE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	_, err := db.ExecContext(ctx, sqlStmt)
	return err
}

func migrateProcedures(ctx context.Context, db *sql.DB) error {
	files, err := sqlproc.ResolveFiles([]string{"examples/backend/sql"})
	if err != nil {
		return err
	}
	parser := sqlproc.NewParser()
	procs, err := parser.ParseFiles(files)
	if err != nil {
		return err
	}
	return sqlproc.NewMigrator(db).Migrate(ctx, procs)
}

func envOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
