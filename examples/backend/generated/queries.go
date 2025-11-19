package generated

import "context"

func (q *Queries) CreateUser(ctx context.Context, name string, email string) (CreateUserRow, error) {
	query := "SELECT * FROM create_user($1, $2)"
	row := q.db.QueryRowContext(ctx, query, name, email)
	var dest CreateUserRow
	if err := row.Scan(&dest.Id, &dest.Name, &dest.Email, &dest.CreatedAt); err != nil {
		return dest, err
	}
	return dest, nil
}

func (q *Queries) DeleteUser(ctx context.Context, userId int32) error {
	query := "SELECT delete_user($1)"
	_, err := q.db.ExecContext(ctx, query, userId)
	return err
}

func (q *Queries) GetUser(ctx context.Context, userId int32) (GetUserRow, error) {
	query := "SELECT * FROM get_user($1)"
	row := q.db.QueryRowContext(ctx, query, userId)
	var dest GetUserRow
	if err := row.Scan(&dest.Id, &dest.Name, &dest.Email, &dest.CreatedAt); err != nil {
		return dest, err
	}
	return dest, nil
}

func (q *Queries) ListUsers(ctx context.Context) ([]ListUsersRow, error) {
	query := "SELECT * FROM list_users()"
	rows, err := q.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]ListUsersRow, 0)
	for rows.Next() {
		var dest ListUsersRow
		if err := rows.Scan(&dest.Id, &dest.Name, &dest.Email, &dest.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, dest)
	}
	return result, rows.Err()
}

func (q *Queries) UpdateUser(ctx context.Context, userId int32, email string) (UpdateUserRow, error) {
	query := "SELECT * FROM update_user($1, $2)"
	row := q.db.QueryRowContext(ctx, query, userId, email)
	var dest UpdateUserRow
	if err := row.Scan(&dest.Id, &dest.Name, &dest.Email, &dest.CreatedAt); err != nil {
		return dest, err
	}
	return dest, nil
}
