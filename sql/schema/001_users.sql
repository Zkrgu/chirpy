-- +goose Up
CREATE TABLE users(
	id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
	created_at TIMESTAMP NOT NULL DEFAULT now(),
	updated_at TIMESTAMP NOT NULL DEFAULT now(),
	email TEXT NOT NULL UNIQUE
);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_modified_column()
	RETURNS TRIGGER AS $$
	BEGIN
		NEW.updated_at = now();
		RETURN NEW;
	END;
	$$ LANGUAGE 'plpgsql';
-- +goose StatementEnd

CREATE TRIGGER update_modified_time BEFORE UPDATE ON users FOR EACH ROW EXECUTE PROCEDURE update_modified_column();

-- +goose Down
DROP TABLE users;
