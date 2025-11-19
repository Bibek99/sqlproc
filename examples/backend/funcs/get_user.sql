-- name: GetUser :one
-- param: user_id int
-- returns: id int, name text, email text, created_at timestamptz

CREATE OR REPLACE FUNCTION get_user(p_user_id INT)
RETURNS TABLE(id INT, name TEXT, email TEXT, created_at TIMESTAMPTZ) AS $$
BEGIN
    RETURN QUERY
    SELECT u.id, u.name, u.email, u.created_at
    FROM users u
    WHERE u.id = p_user_id;
END;
$$ LANGUAGE plpgsql;

