CREATE TABLE IF NOT EXISTS schema_migrations (
                                                 version INTEGER PRIMARY KEY,
                                                 applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
