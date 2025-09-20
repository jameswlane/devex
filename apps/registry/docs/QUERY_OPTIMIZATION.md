# Query Optimization Guide

This document outlines the query optimization strategies implemented in the DevEx Registry to improve database performance.

## Composite Indexes

### Applications Table

#### `idx_applications_official_default_name`
- **Columns**: `official DESC, default DESC, name ASC`
- **Usage**: Registry service pagination queries with official/default ordering
- **Query Pattern**: 
  ```sql
  ORDER BY official DESC, default DESC, name ASC
  ```

#### `idx_applications_category_official`
- **Columns**: `category, official DESC`
- **Usage**: Browsing applications by category with official apps first
- **Query Pattern**:
  ```sql
  WHERE category = ? ORDER BY official DESC
  ```

### Plugins Table

#### `idx_plugins_type_status`
- **Columns**: `type, status`
- **Usage**: Filtering active plugins by type (most common query pattern)
- **Query Pattern**:
  ```sql
  WHERE type = ? AND status = 'active'
  ```

#### `idx_plugins_status_priority_name`
- **Columns**: `status, priority DESC, name ASC`
- **Usage**: Registry service queries for active plugins with priority ordering
- **Query Pattern**:
  ```sql
  WHERE status = 'active' ORDER BY priority DESC, name ASC
  ```

#### `idx_plugins_type_priority`
- **Columns**: `type, priority DESC`
- **Usage**: Plugin type browsing with priority ordering
- **Query Pattern**:
  ```sql
  WHERE type = ? ORDER BY priority DESC
  ```

#### `idx_plugins_downloads_recent`
- **Columns**: `downloadCount DESC, lastDownload DESC`
- **Usage**: Popular plugins queries
- **Query Pattern**:
  ```sql
  ORDER BY downloadCount DESC, lastDownload DESC
  ```

### Configs Table

#### `idx_configs_category_type`
- **Columns**: `category, type`
- **Usage**: Config browsing by category and type
- **Query Pattern**:
  ```sql
  WHERE category = ? AND type = ?
  ```

#### `idx_configs_category_downloads`
- **Columns**: `category, downloadCount DESC`
- **Usage**: Popular configs by category
- **Query Pattern**:
  ```sql
  WHERE category = ? ORDER BY downloadCount DESC
  ```

### JSON Platform Queries

#### Applications Platform Support
The migration includes specialized GIN indexes for JSON platform queries:
- `idx_applications_platforms_linux`
- `idx_applications_platforms_macos`  
- `idx_applications_platforms_windows`

**Usage**: Filtering applications by platform support
```typescript
where: {
  platforms: {
    path: ['linux'],
    not: Prisma.JsonNull
  }
}
```

## Full-Text Search Optimization

### GIN Indexes for Search
- `idx_applications_search_name`: Full-text search on application names
- `idx_applications_search_desc`: Full-text search on application descriptions
- `idx_plugins_search_name`: Full-text search on plugin names
- `idx_plugins_search_desc`: Full-text search on plugin descriptions

**Usage**: PostgreSQL full-text search with `to_tsvector`
```sql
WHERE to_tsvector('english', name) @@ to_tsquery('english', ?)
```

## Query Pattern Best Practices

### 1. Use Composite Indexes for Multi-Column Queries
```typescript
// Good: Uses idx_plugins_type_status
const plugins = await prisma.plugin.findMany({
  where: { 
    type: 'package-manager',
    status: 'active'
  }
});

// Good: Uses idx_applications_official_default_name
const apps = await prisma.application.findMany({
  orderBy: [
    { official: 'desc' },
    { default: 'desc' },
    { name: 'asc' }
  ]
});
```

### 2. Leverage JSON Indexes for Platform Queries
```typescript
// Good: Uses specialized GIN indexes
const linuxApps = await prisma.application.findMany({
  where: {
    platforms: {
      path: ['linux'],
      not: Prisma.JsonNull
    }
  }
});
```

### 3. Optimize Popular Items Queries
```typescript
// Good: Uses download tracking indexes
const popularPlugins = await prisma.plugin.findMany({
  where: { downloadCount: { gt: 0 } },
  orderBy: [
    { downloadCount: 'desc' },
    { lastDownload: 'desc' }
  ]
});
```

## Index Maintenance

### CONCURRENTLY Creation
All indexes are created with `CONCURRENTLY` to avoid blocking operations during migration:
```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS "idx_name" ...
```

### Partial Indexes
Several indexes use `WHERE` clauses to reduce size and improve performance:
```sql
-- Only index active plugins
WHERE "status" = 'active'

-- Only index items with downloads
WHERE "downloadCount" > 0
```

## Monitoring Query Performance

### Using EXPLAIN ANALYZE
```sql
EXPLAIN ANALYZE SELECT * FROM applications 
WHERE category = 'development' AND official = true 
ORDER BY name;
```

### Index Usage Statistics
```sql
SELECT schemaname, tablename, indexname, idx_tup_read, idx_tup_fetch
FROM pg_stat_user_indexes 
ORDER BY idx_tup_read DESC;
```

## Performance Considerations

1. **Index Selectivity**: Composite indexes work best when the first column has high selectivity
2. **Query Planning**: PostgreSQL will choose the most efficient index based on statistics
3. **Index Size**: Monitor index sizes to ensure they fit in memory for optimal performance
4. **Update Performance**: More indexes mean slower writes - balance read vs write performance

## Migration Safety

- All indexes created with `CONCURRENTLY` for zero-downtime deployment
- `IF NOT EXISTS` clauses prevent duplicate creation errors
- Partial indexes reduce storage overhead and improve performance
- No existing data modification required