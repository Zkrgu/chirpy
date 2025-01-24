-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token, user_id, expires_at)
VALUES (
    $1,
	$2,
	$3
)
RETURNING *;

-- name: GetToken :one
SELECT * FROM refresh_tokens WHERE token = $1 AND revoked_at IS NULL AND expires_at > now();

-- name: RevokeToken :exec
UPDATE refresh_tokens SET revoked_at = now() WHERE token = $1;
