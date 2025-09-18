-- Migration: Add composite indexes for query optimization
-- This migration adds composite indexes for common query patterns to improve performance

-- Applications composite indexes
-- 1. Category + platforms JSON path queries (frequently used together)
CREATE INDEX IF NOT EXISTS "idx_applications_category_platforms"
ON "applications"("category")
WHERE ("platforms" IS NOT NULL AND "platforms" != 'null');

-- 2. Official + default + name ordering pattern (registry service)
CREATE INDEX IF NOT EXISTS "idx_applications_official_default_name"
ON "applications"("official" DESC, "default" DESC, "name" ASC);

-- 3. Category + official for filtered browsing
CREATE INDEX IF NOT EXISTS "idx_applications_category_official"
ON "applications"("category", "official" DESC);

-- Plugins composite indexes
-- 4. Type + status for filtered plugin queries (most common pattern)
CREATE INDEX IF NOT EXISTS "idx_plugins_type_status_active"
ON "plugins"("type", "status")
WHERE "status" = 'active';

-- 5. Status + priority + name for ordered active plugin queries
CREATE INDEX IF NOT EXISTS "idx_plugins_status_priority_name"
ON "plugins"("status", "priority" DESC, "name" ASC)
WHERE "status" = 'active';

-- 6. Type + priority for plugin type browsing with priority ordering
CREATE INDEX IF NOT EXISTS "idx_plugins_type_priority"
ON "plugins"("type", "priority" DESC);

-- Configs composite indexes
-- 7. Category + type for config browsing
CREATE INDEX IF NOT EXISTS "idx_configs_category_type"
ON "configs"("category", "type");

-- 8. Category + downloadCount for popular configs by category
CREATE INDEX IF NOT EXISTS "idx_configs_category_downloads"
ON "configs"("category", "downloadCount" DESC)
WHERE "downloadCount" > 0;

-- Stacks composite indexes
-- 9. Category + downloadCount for popular stacks by category
CREATE INDEX IF NOT EXISTS "idx_stacks_category_downloads"
ON "stacks"("category", "downloadCount" DESC)
WHERE "downloadCount" > 0;

-- Cross-table popularity queries
-- 10. Download tracking indexes for all resources
CREATE INDEX IF NOT EXISTS "idx_plugins_downloads_recent"
ON "plugins"("downloadCount" DESC, "lastDownload" DESC NULLS LAST)
WHERE "downloadCount" > 0;

CREATE INDEX IF NOT EXISTS "idx_configs_downloads_recent"
ON "configs"("downloadCount" DESC, "lastDownload" DESC NULLS LAST)
WHERE "downloadCount" > 0;

CREATE INDEX IF NOT EXISTS "idx_stacks_downloads_recent"
ON "stacks"("downloadCount" DESC, "lastDownload" DESC NULLS LAST)
WHERE "downloadCount" > 0;

-- Search optimization indexes
-- 11. Applications search optimization (name + description)
CREATE INDEX IF NOT EXISTS "idx_applications_search_name"
ON "applications" USING gin(to_tsvector('english', "name"));

CREATE INDEX IF NOT EXISTS "idx_applications_search_desc"
ON "applications" USING gin(to_tsvector('english', "description"));

-- 12. Plugins search optimization
CREATE INDEX IF NOT EXISTS "idx_plugins_search_name"
ON "plugins" USING gin(to_tsvector('english', "name"));

CREATE INDEX IF NOT EXISTS "idx_plugins_search_desc"
ON "plugins" USING gin(to_tsvector('english', "description"));

-- Platform filtering optimization for JSON fields
-- 13. B-tree indexes for platform JSON path queries (more suitable for equality checks)
CREATE INDEX IF NOT EXISTS "idx_applications_platforms_linux"
ON "applications"(("platforms"->>'linux'))
WHERE ("platforms"->>'linux') IS NOT NULL;

CREATE INDEX IF NOT EXISTS "idx_applications_platforms_macos"
ON "applications"(("platforms"->>'macos'))
WHERE ("platforms"->>'macos') IS NOT NULL;

CREATE INDEX IF NOT EXISTS "idx_applications_platforms_windows"
ON "applications"(("platforms"->>'windows'))
WHERE ("platforms"->>'windows') IS NOT NULL;

-- Sync performance indexes
-- 14. SyncLog performance for monitoring queries
CREATE INDEX IF NOT EXISTS "idx_sync_logs_type_created"
ON "sync_logs"("type", "createdAt" DESC);

CREATE INDEX IF NOT EXISTS "idx_sync_logs_success_created"
ON "sync_logs"("success", "createdAt" DESC);