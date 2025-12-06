package generated

import "time"

type SqlprocSchemaMigrations struct {
	AppliedAt time.Time `db:"applied_at" json:"appliedAt"`
	Name      string    `db:"name" json:"name"`
	Version   int32     `db:"version" json:"version"`
}

type Users struct {
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	Email     string    `db:"email" json:"email"`
	Id        int32     `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
}
