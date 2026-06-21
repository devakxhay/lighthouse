CREATE TABLE IF NOT EXISTS runtimes (
    id          INTEGER PRIMARY KEY,
    name        TEXT UNIQUE NOT NULL,
    bin_path    TEXT NOT NULL DEFAULT '',
    detected_at DATETIME,
    overridden  BOOLEAN DEFAULT FALSE
);
