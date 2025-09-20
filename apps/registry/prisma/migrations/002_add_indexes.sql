-- Add composite indexes for common query patterns

-- Applications table indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_app_category_platform 
ON applications(category, linux_support_id, macos_support_id, windows_support_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_app_official_default 
ON applications(official, "default");

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_app_tags_gin 
ON applications USING gin(tags);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_app_desktop_environments_gin 
ON applications USING gin(desktop_environments);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_app_name_text_search 
ON applications USING gin(to_tsvector('english', name || ' ' || description));

-- Plugins table indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_plugin_type_status 
ON plugins(type, status);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_plugin_priority_status 
ON plugins(priority DESC, status) WHERE status = 'active';

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_plugin_platforms_gin 
ON plugins USING gin(platforms);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_plugin_supports_gin 
ON plugins USING gin(supports);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_plugin_downloads 
ON plugins(download_count DESC, last_download DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_plugin_name_text_search 
ON plugins USING gin(to_tsvector('english', name || ' ' || description));

-- Configs table indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_config_category_type 
ON configs(category, type);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_config_platforms_gin 
ON configs USING gin(platforms);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_config_downloads 
ON configs(download_count DESC, last_download DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_config_name_text_search 
ON configs USING gin(to_tsvector('english', name || ' ' || description));

-- Stacks table indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_stack_category 
ON stacks(category);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_stack_platforms_gin 
ON stacks USING gin(platforms);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_stack_desktop_environments_gin 
ON stacks USING gin(desktop_environments);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_stack_applications_gin 
ON stacks USING gin(applications);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_stack_downloads 
ON stacks(download_count DESC, last_download DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_stack_name_text_search 
ON stacks USING gin(to_tsvector('english', name || ' ' || description));

-- Platform info table indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_platform_info_method_official 
ON platform_info(install_method, official_support);

-- Registry stats table indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_registry_stats_date 
ON registry_stats(date DESC);

-- Sync logs table indexes  
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sync_logs_type_created 
ON sync_logs(type, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sync_logs_success_created 
ON sync_logs(success, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sync_logs_name_type 
ON sync_logs(name, type);

-- Add partial indexes for better performance on filtered queries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_app_official_only 
ON applications(name) WHERE official = true;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_plugin_active_only 
ON plugins(priority DESC, name) WHERE status = 'active';

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_recent_downloads 
ON plugins(last_download DESC) WHERE last_download IS NOT NULL;

-- Add covering indexes to avoid table lookups
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_app_list_covering 
ON applications(name, category, official, "default") 
INCLUDE (description, tags, desktop_environments);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_plugin_list_covering 
ON plugins(name, type, status, priority) 
INCLUDE (description, platforms, download_count);

-- Comment for future maintenance
COMMENT ON INDEX idx_app_category_platform IS 'Optimizes queries filtering by category and platform support';
COMMENT ON INDEX idx_plugin_type_status IS 'Optimizes queries filtering plugins by type and status';
COMMENT ON INDEX idx_app_name_text_search IS 'Enables full-text search on application names and descriptions';
COMMENT ON INDEX idx_plugin_name_text_search IS 'Enables full-text search on plugin names and descriptions';