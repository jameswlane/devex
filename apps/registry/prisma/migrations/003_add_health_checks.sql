-- Add health checks table for database monitoring

CREATE TABLE IF NOT EXISTS health_checks (
    id VARCHAR(255) PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(50) NOT NULL DEFAULT 'ok',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Add index for cleanup queries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_health_checks_timestamp 
ON health_checks(timestamp DESC);

-- Add index for status monitoring
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_health_checks_status_timestamp 
ON health_checks(status, timestamp DESC);

-- Comment for maintenance
COMMENT ON TABLE health_checks IS 'Health monitoring table for database connection and latency testing';
COMMENT ON INDEX idx_health_checks_timestamp IS 'Optimizes cleanup queries and recent status checks';