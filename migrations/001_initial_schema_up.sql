CREATE TABLE IF NOT EXISTS system_data (
                                           key TEXT PRIMARY KEY,
                                           value TEXT NOT NULL,
                                           updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS installed_apps (
                                              id INTEGER PRIMARY KEY AUTOINCREMENT,
                                              app_name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS schema_version (
                                              version INTEGER PRIMARY KEY,
                                              updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO schema_version (version)
SELECT 1 WHERE NOT EXISTS (SELECT 1 FROM schema_version);
