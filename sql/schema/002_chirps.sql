-- +goose Up
CREATE TABLE chirps(
	id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
	created_at TIMESTAMP NOT NULL DEFAULT now(),
	updated_at TIMESTAMP NOT NULL DEFAULT now(),
	body TEXT NOT NULL,
	user_id UUID NOT NULL REFERENCES users ON DELETE CASCADE
);

CREATE TRIGGER update_modified_time BEFORE UPDATE ON chirps FOR EACH ROW EXECUTE PROCEDURE update_modified_column();

-- +goose Down
DROP TABLE chirps;
