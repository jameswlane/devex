package metrics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// MetricType represents the type of metric being recorded
type MetricType string

const (
	// Installation metrics
	MetricInstallStarted   MetricType = "install.started"
	MetricInstallSucceeded MetricType = "install.succeeded"
	MetricInstallFailed    MetricType = "install.failed"
	MetricInstallSkipped   MetricType = "install.skipped"
	MetricInstallDuration  MetricType = "install.duration"

	// Uninstallation metrics
	MetricUninstallStarted   MetricType = "uninstall.started"
	MetricUninstallSucceeded MetricType = "uninstall.succeeded"
	MetricUninstallFailed    MetricType = "uninstall.failed"
	MetricUninstallDuration  MetricType = "uninstall.duration"

	// Docker specific metrics
	MetricDockerSetupStarted   MetricType = "docker.setup.started"
	MetricDockerSetupSucceeded MetricType = "docker.setup.succeeded"
	MetricDockerSetupFailed    MetricType = "docker.setup.failed"
	MetricDockerSetupSkipped   MetricType = "docker.setup.skipped"
	MetricDockerDaemonReady    MetricType = "docker.daemon.ready"
	MetricDockerGroupAdded     MetricType = "docker.group.added"

	// APT specific metrics
	MetricAPTUpdateStarted   MetricType = "apt.update.started"
	MetricAPTUpdateSucceeded MetricType = "apt.update.succeeded"
	MetricAPTUpdateFailed    MetricType = "apt.update.failed"
	MetricAPTCacheHit        MetricType = "apt.cache.hit"
	MetricAPTCacheMiss       MetricType = "apt.cache.miss"

	// Security metrics
	MetricSecurityValidationFailed  MetricType = "security.validation.failed"
	MetricSecurityValidationSuccess MetricType = "security.validation.success"
	MetricSecurityInjectionBlocked  MetricType = "security.injection.blocked"

	// Performance metrics
	MetricTimeoutOccurred MetricType = "timeout.occurred"
	MetricRetryAttempted  MetricType = "retry.attempted"

	// Container cache metrics
	MetricContainerCacheHit  MetricType = "container.cache.hit"
	MetricContainerCacheMiss MetricType = "container.cache.miss"
)

// Metric represents a single metric event
type Metric struct {
	Type      MetricType
	Timestamp time.Time
	Duration  time.Duration
	Tags      map[string]string
	Value     float64
	Error     error
}

// Collector is the interface for metrics collection
type Collector interface {
	Record(metric Metric)
	RecordCount(metricType MetricType, tags map[string]string)
	RecordDuration(metricType MetricType, duration time.Duration, tags map[string]string)
	RecordError(metricType MetricType, err error, tags map[string]string)
	GetStats() Stats
	Reset()
}

// Stats represents aggregated statistics
type Stats struct {
	TotalInstalls          int64
	SuccessfulInstalls     int64
	FailedInstalls         int64
	SkippedInstalls        int64
	TotalUninstalls        int64
	SuccessfulUninstalls   int64
	FailedUninstalls       int64
	AverageInstallDuration time.Duration
	SecurityBlockedCount   int64
	TimeoutCount           int64
	RetryCount             int64
	SuccessRate            float64
	LastUpdated            time.Time
}

// InMemoryCollector is an in-memory implementation of the Collector interface
type InMemoryCollector struct {
	mu      sync.RWMutex
	metrics []Metric
	stats   Stats
}

// NewInMemoryCollector creates a new in-memory metrics collector
func NewInMemoryCollector() *InMemoryCollector {
	return &InMemoryCollector{
		metrics: make([]Metric, 0, 1000),
		stats:   Stats{},
	}
}

// Record records a metric
func (c *InMemoryCollector) Record(metric Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	metric.Timestamp = time.Now()
	c.metrics = append(c.metrics, metric)

	// Update statistics
	c.updateStats(metric)

	// Log important metrics
	if metric.Error != nil {
		log.Debug("Metric recorded with error", "type", metric.Type, "error", metric.Error, "tags", metric.Tags)
	} else {
		log.Debug("Metric recorded", "type", metric.Type, "tags", metric.Tags)
	}
}

// RecordCount records a count metric
func (c *InMemoryCollector) RecordCount(metricType MetricType, tags map[string]string) {
	c.Record(Metric{
		Type:  metricType,
		Tags:  tags,
		Value: 1,
	})
}

// RecordDuration records a duration metric
func (c *InMemoryCollector) RecordDuration(metricType MetricType, duration time.Duration, tags map[string]string) {
	c.Record(Metric{
		Type:     metricType,
		Duration: duration,
		Tags:     tags,
		Value:    float64(duration.Milliseconds()),
	})
}

// RecordError records an error metric
func (c *InMemoryCollector) RecordError(metricType MetricType, err error, tags map[string]string) {
	c.Record(Metric{
		Type:  metricType,
		Error: err,
		Tags:  tags,
		Value: 1,
	})
}

// GetStats returns current statistics
func (c *InMemoryCollector) GetStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := c.stats
	if stats.TotalInstalls > 0 {
		stats.SuccessRate = float64(stats.SuccessfulInstalls) / float64(stats.TotalInstalls) * 100
	}
	stats.LastUpdated = time.Now()

	return stats
}

// Reset clears all metrics and statistics
func (c *InMemoryCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metrics = make([]Metric, 0, 1000)
	c.stats = Stats{}
}

// updateStats updates internal statistics based on a metric
func (c *InMemoryCollector) updateStats(metric Metric) {
	switch metric.Type {
	case MetricInstallStarted:
		c.stats.TotalInstalls++
	case MetricInstallSucceeded:
		c.stats.SuccessfulInstalls++
	case MetricInstallFailed:
		c.stats.FailedInstalls++
	case MetricInstallSkipped:
		c.stats.SkippedInstalls++
	case MetricInstallDuration:
		c.updateAverageInstallDuration(metric.Duration)
	case MetricUninstallStarted:
		c.stats.TotalUninstalls++
	case MetricUninstallSucceeded:
		c.stats.SuccessfulUninstalls++
	case MetricUninstallFailed:
		c.stats.FailedUninstalls++
	case MetricSecurityInjectionBlocked:
		c.stats.SecurityBlockedCount++
	case MetricTimeoutOccurred:
		c.stats.TimeoutCount++
	case MetricRetryAttempted:
		c.stats.RetryCount++
	}
}

// updateAverageInstallDuration updates the average installation duration
func (c *InMemoryCollector) updateAverageInstallDuration(newDuration time.Duration) {
	if c.stats.SuccessfulInstalls == 0 {
		c.stats.AverageInstallDuration = newDuration
		return
	}

	// Calculate running average
	total := c.stats.AverageInstallDuration * time.Duration(c.stats.SuccessfulInstalls)
	total += newDuration
	c.stats.AverageInstallDuration = total / time.Duration(c.stats.SuccessfulInstalls+1)
}

// Global metrics collector instance
var globalCollector Collector = NewInMemoryCollector()

// SetGlobalCollector sets the global metrics collector
func SetGlobalCollector(collector Collector) {
	globalCollector = collector
}

// Record records a metric using the global collector
func Record(metric Metric) {
	if globalCollector != nil {
		globalCollector.Record(metric)
	}
}

// RecordCount records a count metric using the global collector
func RecordCount(metricType MetricType, tags map[string]string) {
	if globalCollector != nil {
		globalCollector.RecordCount(metricType, tags)
	}
}

// RecordDuration records a duration metric using the global collector
func RecordDuration(metricType MetricType, duration time.Duration, tags map[string]string) {
	if globalCollector != nil {
		globalCollector.RecordDuration(metricType, duration, tags)
	}
}

// RecordError records an error metric using the global collector
func RecordError(metricType MetricType, err error, tags map[string]string) {
	if globalCollector != nil {
		globalCollector.RecordError(metricType, err, tags)
	}
}

// GetStats returns current statistics from the global collector
func GetStats() Stats {
	if globalCollector != nil {
		return globalCollector.GetStats()
	}
	return Stats{}
}

// InstallationTimer helps track installation duration
type InstallationTimer struct {
	startTime   time.Time
	installer   string
	packageName string
}

// StartInstallation starts tracking an installation
func StartInstallation(installer, packageName string) *InstallationTimer {
	RecordCount(MetricInstallStarted, map[string]string{
		"installer": installer,
		"package":   packageName,
	})

	return &InstallationTimer{
		startTime:   time.Now(),
		installer:   installer,
		packageName: packageName,
	}
}

// Success records a successful installation
func (t *InstallationTimer) Success() {
	duration := time.Since(t.startTime)
	tags := map[string]string{
		"installer": t.installer,
		"package":   t.packageName,
	}

	RecordCount(MetricInstallSucceeded, tags)
	RecordDuration(MetricInstallDuration, duration, tags)

	log.Info("Installation succeeded",
		"installer", t.installer,
		"package", t.packageName,
		"duration", duration.String())
}

// Failure records a failed installation
func (t *InstallationTimer) Failure(err error) {
	duration := time.Since(t.startTime)
	tags := map[string]string{
		"installer": t.installer,
		"package":   t.packageName,
	}

	RecordError(MetricInstallFailed, err, tags)
	RecordDuration(MetricInstallDuration, duration, tags)

	log.Error("Installation failed", err,
		"installer", t.installer,
		"package", t.packageName,
		"duration", duration.String())
}

// Skip records a skipped installation
func (t *InstallationTimer) Skip(reason string) {
	duration := time.Since(t.startTime)
	tags := map[string]string{
		"installer": t.installer,
		"package":   t.packageName,
		"reason":    reason,
	}

	RecordCount(MetricInstallSkipped, tags)
	RecordDuration(MetricInstallDuration, duration, tags)

	log.Info("Installation skipped",
		"installer", t.installer,
		"package", t.packageName,
		"reason", reason,
		"duration", duration.String())
}

// TrackOperation is a helper function to track any operation with metrics
func TrackOperation(ctx context.Context, operation string, fn func() error) error {
	startTime := time.Now()
	tags := map[string]string{"operation": operation}

	// Record start
	RecordCount(MetricType(operation+".started"), tags)

	// Execute the operation
	err := fn()

	// Record result
	duration := time.Since(startTime)
	if err != nil {
		RecordError(MetricType(operation+".failed"), err, tags)
		log.Debug("Operation failed", "operation", operation, "duration", duration, "error", err)
	} else {
		RecordCount(MetricType(operation+".succeeded"), tags)
		log.Debug("Operation succeeded", "operation", operation, "duration", duration)
	}

	RecordDuration(MetricType(operation+".duration"), duration, tags)

	return err
}

// ReportStats generates a human-readable report of current statistics
func ReportStats() string {
	stats := GetStats()

	return fmt.Sprintf(`
=== DevEx Installation Statistics ===
Total Installations: %d
Successful: %d
Failed: %d
Success Rate: %.2f%%

Total Uninstallations: %d
Successful: %d
Failed: %d

Average Install Duration: %s
Security Blocks: %d
Timeouts: %d
Retries: %d

Last Updated: %s
=====================================`,
		stats.TotalInstalls,
		stats.SuccessfulInstalls,
		stats.FailedInstalls,
		stats.SuccessRate,
		stats.TotalUninstalls,
		stats.SuccessfulUninstalls,
		stats.FailedUninstalls,
		stats.AverageInstallDuration,
		stats.SecurityBlockedCount,
		stats.TimeoutCount,
		stats.RetryCount,
		stats.LastUpdated.Format(time.RFC3339))
}
