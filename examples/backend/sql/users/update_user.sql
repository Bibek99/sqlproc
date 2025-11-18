-- name: UpdateUser :one
-- param: user_id int
-- param: email text
-- returns: id int, name text, email text, created_at timestamp

CREATE OR REPLACE FUNCTION update_user(p_user_id INT, p_email TEXT)
RETURNS TABLE(id INT, name TEXT, email TEXT, created_at TIMESTAMP) AS $$
BEGIN
    RETURN QUERY
    UPDATE users
    SET email = p_email
    WHERE id = p_user_id
    RETURNING users.id, users.name, users.email, users.created_at;
END;
$$ LANGUAGE plpgsql;

