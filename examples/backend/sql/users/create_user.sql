-- name: CreateUser :one
-- param: name text
-- param: email text
-- returns: id int, name text, email text, created_at timestamp

CREATE OR REPLACE FUNCTION create_user(p_name TEXT, p_email TEXT)
RETURNS TABLE(id INT, name TEXT, email TEXT, created_at TIMESTAMP) AS $$
BEGIN
    RETURN QUERY
    INSERT INTO users (name, email)
    VALUES (p_name, p_email)
    RETURNING users.id, users.name, users.email, users.created_at;
END;
$$ LANGUAGE plpgsql;

