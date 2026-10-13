CREATE TABLE users (
    id uuid PRIMARY KEY,
    email text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    is_admin boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE files (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users(id),
    name text NOT NULL CHECK (length(name) > 0),
    storage_key text NOT NULL UNIQUE,
    media_type text NOT NULL DEFAULT 'application/octet-stream',
    size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX files_owner_created_idx ON files(owner_id, created_at DESC);

CREATE TABLE documents (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users(id),
    title text NOT NULL,
    content jsonb NOT NULL DEFAULT '{}'::jsonb,
    format_version integer NOT NULL DEFAULT 1 CHECK (format_version > 0),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX documents_owner_idx ON documents(owner_id);

CREATE TABLE photos (
    file_id uuid PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
    taken_at timestamptz,
    width integer CHECK (width > 0),
    height integer CHECK (height > 0),
    thumbnail_key text UNIQUE,
    processing_status text NOT NULL DEFAULT 'pending'
        CHECK (processing_status IN ('pending', 'ready', 'failed'))
);
