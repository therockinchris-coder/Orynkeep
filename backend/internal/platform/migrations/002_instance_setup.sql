CREATE TABLE instance_setup (
    id integer PRIMARY KEY CHECK (id = 1),
    administrator_id uuid NOT NULL REFERENCES users(id),
    completed_at timestamptz NOT NULL DEFAULT now()
);

-- Preserve setup completion for installations with an existing administrator.
INSERT INTO instance_setup (id, administrator_id)
SELECT 1, id FROM users WHERE is_admin ORDER BY created_at, id LIMIT 1;
