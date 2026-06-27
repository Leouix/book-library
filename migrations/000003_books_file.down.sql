ALTER TABLE books
  DROP COLUMN file_url,
  DROP COLUMN s3_key,
  DROP COLUMN file_name;

CREATE TABLE IF NOT EXISTS files (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_name TEXT NOT NULL,
    s3_key        TEXT NOT NULL UNIQUE,
    mime_type     TEXT NOT NULL DEFAULT 'text/plain',
    size          BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
