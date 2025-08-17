package status

import "time"

// AppStatus represents the status of an installed application
type AppStatus struct {
	Name              string              `json:"name"`
	Installed         bool                `json:"installed"`
	Version           string              `json:"version,omitempty"`
	LatestVersion     string              `json:"latest_version,omitempty"`
	InstallMethod     string              `json:"install_method,omitempty"`
	InstallDate       *time.Time          `json:"install_date,omitempty"`
	Status            string              `json:"status"` // "healthy", "warning", "error", "not_installed"
	Issues            []string            `json:"issues,omitempty"`
	Dependencies      []DependencyStatus  `json:"dependencies,omitempty"`
	Services          []ServiceStatus     `json:"services,omitempty"`
	PathStatus        bool                `json:"in_path"`
	ConfigStatus      bool                `json:"config_valid"`
	HealthCheckResult string              `json:"health_check"`
	Performance       *PerformanceMetrics `json:"performance,omitempty"`
}

// DependencyStatus represents the status of a dependency
type DependencyStatus struct {
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
}

// ServiceStatus represents the status of a system service
type ServiceStatus struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
	Status string `json:"status"`
}

// PerformanceMetrics represents performance information for an application
type PerformanceMetrics struct {
	CPUUsage    float64 `json:"cpu_usage,omitempty"`    // CPU usage percentage
	MemoryUsage int64   `json:"memory_usage,omitempty"` // Memory usage in bytes
	ProcessID   int     `json:"process_id,omitempty"`   // Process ID if running
	Uptime      string  `json:"uptime,omitempty"`       // How long the process has been running
}
