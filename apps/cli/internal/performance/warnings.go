package performance

import (
	"fmt"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/cache"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
)

// WarningLevel represents the severity of a performance warning
type WarningLevel int

const (
	WarningLevelInfo WarningLevel = iota
	WarningLevelCaution
	WarningLevelWarning
	WarningLevelCritical
)

// PerformanceWarning represents a warning about installation performance
type PerformanceWarning struct {
	Level       WarningLevel
	Application string
	Message     string
	Details     []string
	Suggestions []string
	Metrics     *PerformanceMetrics
}

// PerformanceMetrics contains metrics that trigger warnings
type PerformanceMetrics struct {
	EstimatedSize         int64
	EstimatedDownloadTime time.Duration
	EstimatedInstallTime  time.Duration
	DependencyCount       int
	HistoricalFailureRate float64
	RequiresRestart       bool
	SystemImpact          string
}

// WarningThresholds defines when to trigger warnings
type WarningThresholds struct {
	// Size thresholds in bytes
	LargeSizeThreshold   int64 // 100MB
	HugeSizeThreshold    int64 // 500MB
	MassiveSizeThreshold int64 // 1GB

	// Time thresholds
	LongInstallThreshold     time.Duration // 5 minutes
	VeryLongInstallThreshold time.Duration // 15 minutes

	// Dependency thresholds
	ManyDependenciesThreshold    int // 10 dependencies
	TooManyDependenciesThreshold int // 25 dependencies

	// Failure rate thresholds
	HighFailureRateThreshold     float64 // 10% failure rate
	CriticalFailureRateThreshold float64 // 25% failure rate
}

// DefaultWarningThresholds returns default thresholds
var DefaultWarningThresholds = WarningThresholds{
	LargeSizeThreshold:   100 * 1024 * 1024,  // 100MB
	HugeSizeThreshold:    500 * 1024 * 1024,  // 500MB
	MassiveSizeThreshold: 1024 * 1024 * 1024, // 1GB

	LongInstallThreshold:     5 * time.Minute,
	VeryLongInstallThreshold: 15 * time.Minute,

	ManyDependenciesThreshold:    10,
	TooManyDependenciesThreshold: 25,

	HighFailureRateThreshold:     0.10,
	CriticalFailureRateThreshold: 0.25,
}

// PerformanceAnalyzer analyzes installation performance and generates warnings
type PerformanceAnalyzer struct {
	cacheManager *cache.CacheManager
	thresholds   WarningThresholds
	settings     config.CrossPlatformSettings
}

// NewPerformanceAnalyzer creates a new performance analyzer
func NewPerformanceAnalyzer(settings config.CrossPlatformSettings) (*PerformanceAnalyzer, error) {
	cacheManager, err := cache.NewCacheManager(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache manager: %w", err)
	}

	return &PerformanceAnalyzer{
		cacheManager: cacheManager,
		thresholds:   DefaultWarningThresholds,
		settings:     settings,
	}, nil
}

// AnalyzePreInstall analyzes an application before installation and generates warnings
func (p *PerformanceAnalyzer) AnalyzePreInstall(appName string, appConfig interface{}) []PerformanceWarning {
	var warnings []PerformanceWarning

	// Get historical metrics from cache
	historicalMetrics, err := p.cacheManager.GetPerformanceMetrics(appName, 10)
	if err != nil {
		log.Debug("Failed to get historical metrics", "app", appName, "error", err)
	}

	// Calculate performance metrics
	metrics := p.calculateMetrics(appName, appConfig, historicalMetrics)

	// Check for size warnings
	if sizeWarning := p.checkSizeWarnings(appName, metrics); sizeWarning != nil {
		warnings = append(warnings, *sizeWarning)
	}

	// Check for time warnings
	if timeWarning := p.checkTimeWarnings(appName, metrics); timeWarning != nil {
		warnings = append(warnings, *timeWarning)
	}

	// Check for dependency warnings
	if depWarning := p.checkDependencyWarnings(appName, metrics); depWarning != nil {
		warnings = append(warnings, *depWarning)
	}

	// Check for failure rate warnings
	if failureWarning := p.checkFailureRateWarnings(appName, metrics, historicalMetrics); failureWarning != nil {
		warnings = append(warnings, *failureWarning)
	}

	// Check for system impact warnings
	if impactWarning := p.checkSystemImpactWarnings(appName, metrics); impactWarning != nil {
		warnings = append(warnings, *impactWarning)
	}

	// Ensure we always return a slice, even if empty
	if warnings == nil {
		warnings = []PerformanceWarning{}
	}

	return warnings
}

// AnalyzePostInstall analyzes performance after installation and records metrics
func (p *PerformanceAnalyzer) AnalyzePostInstall(appName string, startTime time.Time, success bool, downloadSize int64) error {
	installTime := time.Since(startTime)

	// Detect current platform
	currentPlatform := platform.DetectPlatform()

	// Record performance metrics
	metrics := &cache.PerformanceMetrics{
		ApplicationName: appName,
		InstallMethod:   "devex",
		Platform:        currentPlatform.OS,
		InstallTime:     installTime,
		TotalTime:       installTime,
		PackageSize:     downloadSize,
		Success:         success,
		Timestamp:       time.Now(),
	}

	if err := p.cacheManager.RecordPerformanceMetrics(metrics); err != nil {
		return fmt.Errorf("failed to record performance metrics: %w", err)
	}

	// Generate post-install warnings if installation was slow or failed
	if !success || installTime > p.thresholds.LongInstallThreshold {
		p.generatePostInstallWarnings(appName, installTime, success)
	}

	return nil
}

// calculateMetrics calculates performance metrics for an application
func (p *PerformanceAnalyzer) calculateMetrics(appName string, appConfig interface{}, historicalMetrics []*cache.PerformanceMetrics) *PerformanceMetrics {
	metrics := &PerformanceMetrics{
		EstimatedSize:         p.estimateSize(appName, appConfig, historicalMetrics),
		EstimatedDownloadTime: p.estimateDownloadTime(appName, historicalMetrics),
		EstimatedInstallTime:  p.estimateInstallTime(appName, historicalMetrics),
		DependencyCount:       p.countDependencies(appConfig),
		HistoricalFailureRate: p.calculateFailureRate(historicalMetrics),
		RequiresRestart:       p.checkRequiresRestart(appName),
		SystemImpact:          p.assessSystemImpact(appName),
	}

	return metrics
}

// checkSizeWarnings checks for size-related warnings
func (p *PerformanceAnalyzer) checkSizeWarnings(appName string, metrics *PerformanceMetrics) *PerformanceWarning {
	if metrics.EstimatedSize >= p.thresholds.MassiveSizeThreshold {
		return &PerformanceWarning{
			Level:       WarningLevelCritical,
			Application: appName,
			Message:     fmt.Sprintf("%s is a very large application (estimated %s)", appName, formatBytes(metrics.EstimatedSize)),
			Details: []string{
				fmt.Sprintf("Download size: approximately %s", formatBytes(metrics.EstimatedSize)),
				"This installation will require significant disk space",
				"Download may take considerable time on slower connections",
			},
			Suggestions: []string{
				"Ensure you have sufficient disk space available",
				"Consider using a faster internet connection",
				"Be prepared for a longer installation time",
			},
			Metrics: metrics,
		}
	}

	if metrics.EstimatedSize >= p.thresholds.HugeSizeThreshold {
		return &PerformanceWarning{
			Level:       WarningLevelWarning,
			Application: appName,
			Message:     fmt.Sprintf("%s is a large application (estimated %s)", appName, formatBytes(metrics.EstimatedSize)),
			Details: []string{
				fmt.Sprintf("Download size: approximately %s", formatBytes(metrics.EstimatedSize)),
				"This will require substantial disk space",
			},
			Suggestions: []string{
				"Check available disk space before proceeding",
				"Installation may take several minutes",
			},
			Metrics: metrics,
		}
	}

	if metrics.EstimatedSize >= p.thresholds.LargeSizeThreshold {
		return &PerformanceWarning{
			Level:       WarningLevelCaution,
			Application: appName,
			Message:     fmt.Sprintf("%s is moderately large (%s)", appName, formatBytes(metrics.EstimatedSize)),
			Details: []string{
				fmt.Sprintf("Download size: approximately %s", formatBytes(metrics.EstimatedSize)),
			},
			Suggestions: []string{
				"Installation may take a few minutes",
			},
			Metrics: metrics,
		}
	}

	return nil
}

// checkTimeWarnings checks for time-related warnings
func (p *PerformanceAnalyzer) checkTimeWarnings(appName string, metrics *PerformanceMetrics) *PerformanceWarning {
	totalTime := metrics.EstimatedDownloadTime + metrics.EstimatedInstallTime

	if totalTime >= p.thresholds.VeryLongInstallThreshold {
		return &PerformanceWarning{
			Level:       WarningLevelWarning,
			Application: appName,
			Message:     fmt.Sprintf("%s installation may take a long time (estimated %s)", appName, formatDuration(totalTime)),
			Details: []string{
				fmt.Sprintf("Estimated download time: %s", formatDuration(metrics.EstimatedDownloadTime)),
				fmt.Sprintf("Estimated installation time: %s", formatDuration(metrics.EstimatedInstallTime)),
				"This is based on historical installation data",
			},
			Suggestions: []string{
				"Consider running this installation during a break",
				"Ensure stable internet connection",
				"You can continue using your system during installation",
			},
			Metrics: metrics,
		}
	}

	if totalTime >= p.thresholds.LongInstallThreshold {
		return &PerformanceWarning{
			Level:       WarningLevelInfo,
			Application: appName,
			Message:     fmt.Sprintf("%s installation estimated at %s", appName, formatDuration(totalTime)),
			Details: []string{
				"Installation time based on system performance",
			},
			Suggestions: []string{
				"Installation will proceed in the background",
			},
			Metrics: metrics,
		}
	}

	return nil
}

// checkDependencyWarnings checks for dependency-related warnings
func (p *PerformanceAnalyzer) checkDependencyWarnings(appName string, metrics *PerformanceMetrics) *PerformanceWarning {
	if metrics.DependencyCount >= p.thresholds.TooManyDependenciesThreshold {
		return &PerformanceWarning{
			Level:       WarningLevelWarning,
			Application: appName,
			Message:     fmt.Sprintf("%s has many dependencies (%d packages)", appName, metrics.DependencyCount),
			Details: []string{
				fmt.Sprintf("Will install %d additional packages", metrics.DependencyCount),
				"This may significantly increase installation time",
				"Additional disk space will be required for dependencies",
			},
			Suggestions: []string{
				"Review the dependency list if prompted",
				"Ensure sufficient disk space for all packages",
				"Consider if all dependencies are needed",
			},
			Metrics: metrics,
		}
	}

	if metrics.DependencyCount >= p.thresholds.ManyDependenciesThreshold {
		return &PerformanceWarning{
			Level:       WarningLevelInfo,
			Application: appName,
			Message:     fmt.Sprintf("%s will install %d dependencies", appName, metrics.DependencyCount),
			Details: []string{
				"Multiple packages will be installed",
			},
			Suggestions: []string{
				"This is normal for complex applications",
			},
			Metrics: metrics,
		}
	}

	return nil
}

// checkFailureRateWarnings checks for historical failure warnings
func (p *PerformanceAnalyzer) checkFailureRateWarnings(appName string, metrics *PerformanceMetrics, historicalMetrics []*cache.PerformanceMetrics) *PerformanceWarning {
	if len(historicalMetrics) < 3 {
		return nil // Not enough data for reliable failure rate
	}

	if metrics.HistoricalFailureRate >= p.thresholds.CriticalFailureRateThreshold {
		return &PerformanceWarning{
			Level:       WarningLevelCritical,
			Application: appName,
			Message:     fmt.Sprintf("%s has a high failure rate (%.0f%% of recent installations failed)", appName, metrics.HistoricalFailureRate*100),
			Details: []string{
				"Recent installations have frequently failed",
				"This may indicate compatibility or configuration issues",
				p.getFailureReasons(historicalMetrics),
			},
			Suggestions: []string{
				"Check system requirements carefully",
				"Review recent error logs",
				"Consider alternative installation methods",
				"Ensure all prerequisites are installed",
			},
			Metrics: metrics,
		}
	}

	if metrics.HistoricalFailureRate >= p.thresholds.HighFailureRateThreshold {
		return &PerformanceWarning{
			Level:       WarningLevelCaution,
			Application: appName,
			Message:     fmt.Sprintf("%s occasionally fails to install (%.0f%% failure rate)", appName, metrics.HistoricalFailureRate*100),
			Details: []string{
				"Some recent installations have failed",
			},
			Suggestions: []string{
				"Ensure prerequisites are met",
				"Check available disk space",
			},
			Metrics: metrics,
		}
	}

	return nil
}

// checkSystemImpactWarnings checks for system impact warnings
func (p *PerformanceAnalyzer) checkSystemImpactWarnings(appName string, metrics *PerformanceMetrics) *PerformanceWarning {
	if metrics.RequiresRestart {
		return &PerformanceWarning{
			Level:       WarningLevelWarning,
			Application: appName,
			Message:     fmt.Sprintf("%s may require system restart", appName),
			Details: []string{
				"System services may need to be restarted",
				"Some features may not work until restart",
			},
			Suggestions: []string{
				"Save your work before proceeding",
				"Plan for a system restart after installation",
			},
			Metrics: metrics,
		}
	}

	if metrics.SystemImpact == "high" {
		return &PerformanceWarning{
			Level:       WarningLevelCaution,
			Application: appName,
			Message:     fmt.Sprintf("%s is a system-level application", appName),
			Details: []string{
				"Will modify system configuration",
				"May affect other applications",
			},
			Suggestions: []string{
				"Review what changes will be made",
				"Consider creating a system backup",
			},
			Metrics: metrics,
		}
	}

	return nil
}

// Helper methods for metric calculation

func (p *PerformanceAnalyzer) estimateSize(appName string, appConfig interface{}, historicalMetrics []*cache.PerformanceMetrics) int64 {
	// First, check historical data
	if len(historicalMetrics) > 0 {
		var totalSize int64
		var count int
		for _, m := range historicalMetrics {
			if m.PackageSize > 0 {
				totalSize += m.PackageSize
				count++
			}
		}
		if count > 0 {
			return totalSize / int64(count)
		}
	}

	// Use known sizes for common applications
	knownSizes := map[string]int64{
		"docker":         450 * 1024 * 1024,
		"docker-compose": 50 * 1024 * 1024,
		"node":           80 * 1024 * 1024,
		"nodejs":         80 * 1024 * 1024,
		"python":         100 * 1024 * 1024,
		"rust":           250 * 1024 * 1024,
		"go":             350 * 1024 * 1024,
		"java":           180 * 1024 * 1024,
		"vscode":         350 * 1024 * 1024,
		"android-studio": 900 * 1024 * 1024,
		"intellij-idea":  800 * 1024 * 1024,
		"chrome":         200 * 1024 * 1024,
		"firefox":        150 * 1024 * 1024,
		"postgresql":     150 * 1024 * 1024,
		"mysql":          450 * 1024 * 1024,
		"mongodb":        250 * 1024 * 1024,
		"redis":          50 * 1024 * 1024,
		"nginx":          30 * 1024 * 1024,
		"apache2":        50 * 1024 * 1024,
	}

	// Check if we have a known size
	appNameLower := strings.ToLower(appName)
	if size, exists := knownSizes[appNameLower]; exists {
		return size
	}

	// Check for partial matches
	for knownApp, size := range knownSizes {
		if strings.Contains(appNameLower, knownApp) || strings.Contains(knownApp, appNameLower) {
			return size
		}
	}

	// Default estimate for unknown applications
	return 50 * 1024 * 1024 // 50MB default
}

func (p *PerformanceAnalyzer) estimateDownloadTime(appName string, historicalMetrics []*cache.PerformanceMetrics) time.Duration {
	// Use historical data if available
	if len(historicalMetrics) > 0 {
		var totalTime time.Duration
		var count int
		for _, m := range historicalMetrics {
			if m.DownloadTime > 0 {
				totalTime += m.DownloadTime
				count++
			}
		}
		if count > 0 {
			return totalTime / time.Duration(count)
		}
	}

	// Estimate based on size and assumed bandwidth (10 Mbps)
	estimatedSize := p.estimateSize(appName, nil, historicalMetrics)
	bandwidthBytesPerSecond := int64(10 * 1024 * 1024 / 8) // 10 Mbps in bytes/second
	seconds := estimatedSize / bandwidthBytesPerSecond
	return time.Duration(seconds) * time.Second
}

func (p *PerformanceAnalyzer) estimateInstallTime(appName string, historicalMetrics []*cache.PerformanceMetrics) time.Duration {
	// Use historical data if available
	if len(historicalMetrics) > 0 {
		var totalTime time.Duration
		var count int
		for _, m := range historicalMetrics {
			if m.InstallTime > 0 {
				totalTime += m.InstallTime
				count++
			}
		}
		if count > 0 {
			return totalTime / time.Duration(count)
		}
	}

	// Estimate based on application type
	baseTime := 30 * time.Second

	// Adjust for known slow installers
	slowInstallers := map[string]time.Duration{
		"docker":         3 * time.Minute,
		"android-studio": 5 * time.Minute,
		"intellij-idea":  4 * time.Minute,
		"vscode":         2 * time.Minute,
		"postgresql":     2 * time.Minute,
		"mysql":          3 * time.Minute,
		"mongodb":        2 * time.Minute,
	}

	appNameLower := strings.ToLower(appName)
	if duration, exists := slowInstallers[appNameLower]; exists {
		return duration
	}

	return baseTime
}

func (p *PerformanceAnalyzer) countDependencies(appConfig interface{}) int {
	// This would need to parse the app config to count dependencies
	// For now, return estimates based on common patterns
	return 0 // Placeholder
}

func (p *PerformanceAnalyzer) calculateFailureRate(historicalMetrics []*cache.PerformanceMetrics) float64 {
	if len(historicalMetrics) == 0 {
		return 0
	}

	failures := 0
	for _, m := range historicalMetrics {
		if !m.Success {
			failures++
		}
	}

	return float64(failures) / float64(len(historicalMetrics))
}

func (p *PerformanceAnalyzer) checkRequiresRestart(appName string) bool {
	// Applications that typically require restart
	requiresRestart := []string{
		"docker",
		"virtualbox",
		"vmware",
		"kernel",
		"systemd",
		"nvidia-driver",
		"cuda",
	}

	appNameLower := strings.ToLower(appName)
	for _, app := range requiresRestart {
		if strings.Contains(appNameLower, app) {
			return true
		}
	}

	return false
}

func (p *PerformanceAnalyzer) assessSystemImpact(appName string) string {
	// High impact applications
	highImpact := []string{
		"docker",
		"kubernetes",
		"virtualbox",
		"vmware",
		"systemd",
		"kernel",
		"nvidia-driver",
		"firewall",
		"selinux",
	}

	appNameLower := strings.ToLower(appName)
	for _, app := range highImpact {
		if strings.Contains(appNameLower, app) {
			return "high"
		}
	}

	// Medium impact applications
	mediumImpact := []string{
		"postgresql",
		"mysql",
		"mongodb",
		"redis",
		"nginx",
		"apache",
	}

	for _, app := range mediumImpact {
		if strings.Contains(appNameLower, app) {
			return "medium"
		}
	}

	return "low"
}

func (p *PerformanceAnalyzer) getFailureReasons(historicalMetrics []*cache.PerformanceMetrics) string {
	// Analyze recent failures to identify patterns
	reasons := make(map[string]int)

	for _, m := range historicalMetrics {
		if !m.Success && m.ErrorMessage != "" {
			// Categorize error messages
			switch {
			case strings.Contains(m.ErrorMessage, "space"):
				reasons["disk space"]++
			case strings.Contains(m.ErrorMessage, "permission") || strings.Contains(m.ErrorMessage, "denied"):
				reasons["permissions"]++
			case strings.Contains(m.ErrorMessage, "network") || strings.Contains(m.ErrorMessage, "timeout"):
				reasons["network"]++
			case strings.Contains(m.ErrorMessage, "dependency") || strings.Contains(m.ErrorMessage, "require"):
				reasons["dependencies"]++
			default:
				reasons["other"]++
			}
		}
	}

	// Find most common reason
	maxCount := 0
	mostCommon := "unknown issues"
	for reason, count := range reasons {
		if count > maxCount {
			maxCount = count
			mostCommon = reason
		}
	}

	return fmt.Sprintf("Most common failure reason: %s", mostCommon)
}

func (p *PerformanceAnalyzer) generatePostInstallWarnings(appName string, installTime time.Duration, success bool) {
	switch {
	case !success:
		log.Warn("Installation failed", "app", appName, "time", formatDuration(installTime))
		log.Info("💡 Tip: Check the logs for error details and try running with --verbose flag")
	case installTime > p.thresholds.VeryLongInstallThreshold:
		log.Warn("Installation took longer than expected", "app", appName, "time", formatDuration(installTime))
		log.Info("💡 Tip: Future installations may be faster due to caching")
	case installTime > p.thresholds.LongInstallThreshold:
		log.Info("Installation completed", "app", appName, "time", formatDuration(installTime))
	}
}

// FormatWarning formats a warning for display
func FormatWarning(warning PerformanceWarning) string {
	var builder strings.Builder

	// Add emoji based on level
	switch warning.Level {
	case WarningLevelCritical:
		builder.WriteString("🚨 ")
	case WarningLevelWarning:
		builder.WriteString("⚠️  ")
	case WarningLevelCaution:
		builder.WriteString("⚡ ")
	case WarningLevelInfo:
		builder.WriteString("ℹ️  ")
	}

	// Add main message
	builder.WriteString(warning.Message)
	builder.WriteString("\n")

	// Add details if present
	if len(warning.Details) > 0 {
		builder.WriteString("\n📋 Details:\n")
		for _, detail := range warning.Details {
			builder.WriteString(fmt.Sprintf("   • %s\n", detail))
		}
	}

	// Add suggestions if present
	if len(warning.Suggestions) > 0 {
		builder.WriteString("\n💡 Suggestions:\n")
		for _, suggestion := range warning.Suggestions {
			builder.WriteString(fmt.Sprintf("   • %s\n", suggestion))
		}
	}

	return builder.String()
}

// Helper functions

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f seconds", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1f minutes", d.Minutes())
	}
	return fmt.Sprintf("%.1f hours", d.Hours())
}
