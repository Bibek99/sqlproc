-- name: ListUsers :many
-- returns: id int, name text, email text, created_at timestamptz

CREATE OR REPLACE FUNCTION list_users()
RETURNS TABLE(id INT, name TEXT, email TEXT, created_at TIMESTAMPTZ) AS $$
BEGIN
    RETURN QUERY
    SELECT u.id, u.name, u.email, u.created_at
    FROM users u
    ORDER BY u.created_at DESC;
END;
$$ LANGUAGE plpgsql;

