CREATE TABLE IF NOT EXISTS contacts (
  id        TEXT PRIMARY KEY,
  name      TEXT NOT NULL,
  email     TEXT DEFAULT '',
  message   TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS comments (
  id        TEXT PRIMARY KEY,
  parent_id TEXT DEFAULT '',
  post      TEXT NOT NULL,
  name      TEXT NOT NULL,
  email     TEXT NOT NULL,
  comment   TEXT NOT NULL,
  created_at TEXT NOT NULL
);
