CREATE TABLE IF NOT EXISTS apps (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT UNIQUE NOT NULL,
  type        TEXT NOT NULL,
  domain      TEXT NOT NULL,
  port        INTEGER NOT NULL,
  binary_path TEXT NOT NULL DEFAULT '',
  app_dir     TEXT NOT NULL DEFAULT '',
  status      TEXT NOT NULL DEFAULT 'pending',
  created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS certs (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  app_id      INTEGER NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  domain      TEXT NOT NULL,
  issued_at   TIMESTAMP NOT NULL,
  expires_at  TIMESTAMP NOT NULL,
  cert_path   TEXT NOT NULL,
  key_path    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_log (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  timestamp    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  app_name     TEXT NOT NULL,
  action       TEXT NOT NULL,
  status       TEXT NOT NULL,
  triggered_by TEXT NOT NULL DEFAULT 'ui',
  details      TEXT NOT NULL DEFAULT ''
);