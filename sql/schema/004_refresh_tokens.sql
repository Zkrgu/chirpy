-- +goose Up
CREATE TABLE refresh_tokens(
	token TEXT NOT NULL PRIMARY KEY,
	created_at TIMESTAMP NOT NULL DEFAULT now(),
	updated_at TIMESTAMP NOT NULL DEFAULT now(),
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at TIMESTAMP NOT NULL,
	revoked_at TIMESTAMP
);

CREATE TRIGGER update_modified_time BEFORE UPDATE ON refresh_tokens FOR EACH ROW EXECUTE PROCEDURE update_modified_column();

-- +goose Down
DROP TABLE refresh_tokens;
