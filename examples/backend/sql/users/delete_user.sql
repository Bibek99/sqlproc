-- name: DeleteUser :exec
-- param: user_id int

CREATE OR REPLACE FUNCTION delete_user(p_user_id INT)
RETURNS VOID AS $$
BEGIN
    DELETE FROM users WHERE id = p_user_id;
END;
$$ LANGUAGE plpgsql;

