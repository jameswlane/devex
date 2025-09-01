package tui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRepository implements types.Repository for testing
type MockRepository struct {
	installedApps []string
	shouldError   bool
	mutex         sync.Mutex
}

func (m *MockRepository) AddApp(name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldError {
		return fmt.Errorf("mock repository error")
	}

	m.installedApps = append(m.installedApps, name)
	return nil
}

func (m *MockRepository) GetApps() ([]string, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldError {
		return nil, fmt.Errorf("mock repository error")
	}

	return append([]string{}, m.installedApps...), nil
}

func (m *MockRepository) RemoveApp(name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldError {
		return fmt.Errorf("mock repository error")
	}

	for i, app := range m.installedApps {
		if app == name {
			m.installedApps = append(m.installedApps[:i], m.installedApps[i+1:]...)
			break
		}
	}
	return nil
}

// Additional Repository interface methods
func (m *MockRepository) DeleteApp(name string) error {
	return m.RemoveApp(name)
}

func (m *MockRepository) GetApp(name string) (*types.AppConfig, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldError {
		return nil, fmt.Errorf("mock repository error")
	}

	for _, app := range m.installedApps {
		if app == name {
			return &types.AppConfig{
				BaseConfig: types.BaseConfig{Name: name},
			}, nil
		}
	}
	return nil, fmt.Errorf("app not found")
}

func (m *MockRepository) ListApps() ([]types.AppConfig, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldError {
		return nil, fmt.Errorf("mock repository error")
	}

	apps := make([]types.AppConfig, 0, len(m.installedApps))
	for _, name := range m.installedApps {
		apps = append(apps, types.AppConfig{
			BaseConfig: types.BaseConfig{Name: name},
		})
	}
	return apps, nil
}

func (m *MockRepository) SaveApp(app types.AppConfig) error {
	return m.AddApp(app.Name)
}

func (m *MockRepository) Set(key string, value string) error {
	if m.shouldError {
		return fmt.Errorf("mock repository error")
	}
	return nil
}

func (m *MockRepository) Get(key string) (string, error) {
	if m.shouldError {
		return "", fmt.Errorf("mock repository error")
	}
	return "mock-value", nil
}

func (m *MockRepository) IsInstalled(name string) (bool, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldError {
		return false, fmt.Errorf("mock repository error")
	}

	for _, app := range m.installedApps {
		if app == name {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockRepository) Close() error {
	return nil
}

// MockCommandExecutor implements CommandExecutor for testing
type MockCommandExecutor struct {
	shouldError      bool
	validationError  bool
	executedCommands []string
	mutex            sync.Mutex
}

// NewMockCommandExecutor creates a new mock command executor
func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{
		executedCommands: make([]string, 0),
	}
}

// SetShouldError sets whether the executor should return errors
func (m *MockCommandExecutor) SetShouldError(shouldError bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.shouldError = shouldError
}

// SetValidationError sets whether validation should fail
func (m *MockCommandExecutor) SetValidationError(validationError bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.validationError = validationError
}

// GetExecutedCommands returns the list of commands that were executed
func (m *MockCommandExecutor) GetExecutedCommands() []string {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return append([]string{}, m.executedCommands...)
}

// ExecuteCommand implements CommandExecutor.ExecuteCommand with mock behavior
func (m *MockCommandExecutor) ExecuteCommand(ctx context.Context, command string) (*exec.Cmd, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check validation first (simulating the real executor)
	if m.validationError {
		// For dangerous commands, fail validation
		if contains(command, []string{"rm -rf /", "dangerous"}) {
			return nil, fmt.Errorf("mock validation error for dangerous command: %s", command)
		}
	}

	// Record the command for verification
	m.executedCommands = append(m.executedCommands, command)

	if m.shouldError {
		return nil, fmt.Errorf("mock command executor error")
	}

	// Return a fast, silent command that respects context cancellation
	// Use /bin/true which exits immediately with success
	cmd := exec.CommandContext(ctx, "/bin/true")
	return cmd, nil
}

// Helper function to check if command contains any of the dangerous patterns
func contains(command string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(command, pattern) {
			return true
		}
	}
	return false
}

// ValidateCommand implements CommandExecutor.ValidateCommand with mock behavior
func (m *MockCommandExecutor) ValidateCommand(command string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.validationError {
		return fmt.Errorf("mock validation error for dangerous command: %s", command)
	}

	// Allow all commands in tests by default
	return nil
}

func TestStreamingInstaller_Integration(t *testing.T) {
	// Create test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create mock components - use nil program to avoid TUI message sending
	mockRepo := &MockRepository{}
	mockExecutor := NewMockCommandExecutor()
	installer := NewStreamingInstallerWithExecutor(nil, mockRepo, ctx, mockExecutor, getTestSettings())

	// Create test apps with safe commands
	apps := []types.CrossPlatformApp{
		{
			Name:        "test-echo",
			Description: "Test echo command",
			Linux: types.OSConfig{
				InstallMethod:  "generic",
				InstallCommand: "echo 'Installing test-echo'",
			},
		},
		{
			Name:        "test-whoami",
			Description: "Test whoami command",
			Linux: types.OSConfig{
				InstallMethod:  "generic",
				InstallCommand: "whoami",
			},
		},
	}

	settings := config.CrossPlatformSettings{
		Verbose: true,
	}

	// Test successful installation
	err := installer.InstallApps(context.Background(), apps, settings)
	assert.NoError(t, err)

	// Verify apps were added to repository
	installedApps, err := mockRepo.GetApps()
	require.NoError(t, err)
	assert.Contains(t, installedApps, "test-echo")
	assert.Contains(t, installedApps, "test-whoami")
}

func TestStreamingInstaller_CommandValidationIntegration(t *testing.T) {
	ctx := context.Background()
	mockRepo := &MockRepository{}
	mockExecutor := NewMockCommandExecutor()
	// Set up executor to fail validation for dangerous commands
	mockExecutor.SetValidationError(true)
	installer := NewStreamingInstallerWithExecutor(nil, mockRepo, ctx, mockExecutor, getTestSettings())

	// Test app with dangerous command
	dangerousApps := []types.CrossPlatformApp{
		{
			Name:        "dangerous-app",
			Description: "App with dangerous command",
			Linux: types.OSConfig{
				InstallMethod:  "generic",
				InstallCommand: "rm -rf / && echo 'hacked'",
			},
		},
	}

	settings := config.CrossPlatformSettings{}

	// Should succeed overall (InstallApps continues despite individual failures)
	err := installer.InstallApps(context.Background(), dangerousApps, settings)
	assert.NoError(t, err)

	// Verify app was not added to repository
	installedApps, err := mockRepo.GetApps()
	require.NoError(t, err)
	assert.NotContains(t, installedApps, "dangerous-app")
}

func TestStreamingInstaller_ContextCancellationIntegration(t *testing.T) {
	// Test a simple case: cancel context before even starting installation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	mockRepo := &MockRepository{}
	mockExecutor := &MockCommandExecutor{}
	installer := NewStreamingInstallerWithExecutor(nil, mockRepo, ctx, mockExecutor, getTestSettings())

	// Simple app that should be cancelled immediately
	apps := []types.CrossPlatformApp{
		{
			Name:        "test-app",
			Description: "Simple test app",
			Linux: types.OSConfig{
				InstallMethod:  "generic",
				InstallCommand: "echo 'should not execute'",
			},
		},
	}

	settings := config.CrossPlatformSettings{}

	// Call InstallApps with already-cancelled context
	err := installer.InstallApps(ctx, apps, settings)

	// Should get context.Canceled error immediately
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled),
		"Expected context.Canceled, got: %v", err)

	// Commands should not have been executed
	executedCommands := mockExecutor.GetExecutedCommands()
	assert.Empty(t, executedCommands, "No commands should have been executed with cancelled context")
}

func TestStreamingInstaller_RepositoryError(t *testing.T) {
	ctx := context.Background()

	// Create mock repo that always errors
	mockRepo := &MockRepository{shouldError: true}
	mockExecutor := NewMockCommandExecutor()
	installer := NewStreamingInstallerWithExecutor(nil, mockRepo, ctx, mockExecutor, getTestSettings())

	apps := []types.CrossPlatformApp{
		{
			Name:        "test-app",
			Description: "Test application",
			Linux: types.OSConfig{
				InstallMethod:  "generic",
				InstallCommand: "echo 'test'",
			},
		},
	}

	settings := config.CrossPlatformSettings{}

	// Should succeed even with repository error (only logs warning)
	err := installer.InstallApps(context.Background(), apps, settings)
	assert.NoError(t, err)
}

func TestStreamingInstaller_PrePostInstallCommands(t *testing.T) {
	ctx := context.Background()
	mockRepo := &MockRepository{}
	mockExecutor := NewMockCommandExecutor()
	installer := NewStreamingInstallerWithExecutor(nil, mockRepo, ctx, mockExecutor, getTestSettings())

	apps := []types.CrossPlatformApp{
		{
			Name:        "complex-app",
			Description: "App with pre/post install commands",
			Linux: types.OSConfig{
				InstallMethod:  "generic",
				InstallCommand: "echo 'main install'",
				PreInstall: []types.InstallCommand{
					{Command: "echo 'pre-install step 1'"},
					{Command: "echo 'pre-install step 2'"},
				},
				PostInstall: []types.InstallCommand{
					{Command: "echo 'post-install step 1'"},
					{Command: "echo 'post-install step 2'"},
				},
			},
		},
	}

	settings := config.CrossPlatformSettings{}

	err := installer.InstallApps(context.Background(), apps, settings)
	assert.NoError(t, err)

	// Verify app was installed
	installedApps, err := mockRepo.GetApps()
	require.NoError(t, err)
	assert.Contains(t, installedApps, "complex-app")
}

func TestStreamingInstaller_SleepCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mockRepo := &MockRepository{}
	mockExecutor := NewMockCommandExecutor()
	installer := NewStreamingInstallerWithExecutor(nil, mockRepo, ctx, mockExecutor, getTestSettings())

	apps := []types.CrossPlatformApp{
		{
			Name:        "sleep-app",
			Description: "App with sleep command",
			Linux: types.OSConfig{
				InstallMethod:  "generic",
				InstallCommand: "echo 'before sleep'",
				PostInstall: []types.InstallCommand{
					{Sleep: 1}, // 1 second sleep
					{Command: "echo 'after sleep'"},
				},
			},
		},
	}

	settings := config.CrossPlatformSettings{}

	start := time.Now()
	err := installer.InstallApps(context.Background(), apps, settings)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, duration, 1*time.Second)

	// Verify app was installed
	installedApps, err := mockRepo.GetApps()
	require.NoError(t, err)
	assert.Contains(t, installedApps, "sleep-app")
}

func TestStreamingInstaller_SleepCancellation(t *testing.T) {
	// Test context cancellation behavior - simplified to just test early cancellation
	ctx, cancel := context.WithCancel(context.Background())

	mockRepo := &MockRepository{}
	mockExecutor := &MockCommandExecutor{}
	// Don't set validation error for this test
	mockExecutor.SetValidationError(false)
	installer := NewStreamingInstallerWithExecutor(nil, mockRepo, ctx, mockExecutor, getTestSettings())

	// Create a simple app
	apps := []types.CrossPlatformApp{
		{
			Name:        "simple-app",
			Description: "Simple test app",
			Linux: types.OSConfig{
				InstallMethod:  "generic",
				InstallCommand: "echo 'test'",
			},
		},
	}

	settings := config.CrossPlatformSettings{}

	// Cancel context before starting installation
	cancel()

	// Call InstallApps with already-cancelled context
	err := installer.InstallApps(ctx, apps, settings)

	// Should get context.Canceled error immediately
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled),
		"Expected context.Canceled, got: %v", err)

	// Commands should not have been executed
	executedCommands := mockExecutor.GetExecutedCommands()
	assert.Empty(t, executedCommands, "No commands should have been executed with cancelled context")
}

func TestStreamingInstaller_MultipleAppsWithErrors(t *testing.T) {
	ctx := context.Background()
	mockRepo := &MockRepository{}
	mockExecutor := NewMockCommandExecutor()
	// Enable validation error to block dangerous commands
	mockExecutor.SetValidationError(true)
	installer := NewStreamingInstallerWithExecutor(nil, mockRepo, ctx, mockExecutor, getTestSettings())

	apps := []types.CrossPlatformApp{
		{
			Name:        "good-app-1",
			Description: "First good app",
			Linux: types.OSConfig{
				InstallMethod:  "generic",
				InstallCommand: "echo 'good app 1'",
			},
		},
		{
			Name:        "bad-app",
			Description: "App with bad command",
			Linux: types.OSConfig{
				InstallMethod:  "generic",
				InstallCommand: "rm -rf /", // Should be blocked
			},
		},
		{
			Name:        "good-app-2",
			Description: "Second good app",
			Linux: types.OSConfig{
				InstallMethod:  "generic",
				InstallCommand: "echo 'good app 2'",
			},
		},
	}

	settings := config.CrossPlatformSettings{}

	// Should continue with other apps even if one fails
	err := installer.InstallApps(context.Background(), apps, settings)
	assert.NoError(t, err)

	// Verify good apps were installed, bad app was not
	installedApps, err := mockRepo.GetApps()
	require.NoError(t, err)
	assert.Contains(t, installedApps, "good-app-1")
	assert.NotContains(t, installedApps, "bad-app")
	assert.Contains(t, installedApps, "good-app-2")
}

func TestStartInstallation_Integration(t *testing.T) {
	apps := createTestApps()
	mockRepo := &MockRepository{}
	_ = config.CrossPlatformSettings{} // We don't use settings in this test

	// Test that StartInstallation properly sets up and runs
	// Note: This is a minimal test since StartInstallation runs a full TUI
	// In a real scenario, this would be tested with a headless terminal

	// We can't easily test the full TUI interaction, but we can test
	// that the function doesn't panic and sets up correctly
	require.NotPanics(t, func() {
		// Create a context with short timeout to prevent hanging
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Mock the StartInstallation call by creating the components manually
		model := NewModel(apps)
		program := tea.NewProgram(model)
		mockExecutor := NewMockCommandExecutor()
		installer := NewStreamingInstallerWithExecutor(program, mockRepo, ctx, mockExecutor, getTestSettings())

		// Verify installer was created correctly
		assert.NotNil(t, installer)
		assert.Equal(t, mockRepo, installer.repo)
		// Note: installer.ctx is a child context, so we can't directly compare
	})
}

func TestStreamingInstaller_ConcurrentInstallations(t *testing.T) {
	const numInstallations = 3

	ctx := context.Background()
	var wg sync.WaitGroup
	results := make([]error, numInstallations)

	for i := 0; i < numInstallations; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			mockRepo := &MockRepository{}
			mockExecutor := NewMockCommandExecutor()
			installer := NewStreamingInstallerWithExecutor(nil, mockRepo, ctx, mockExecutor, getTestSettings())

			apps := []types.CrossPlatformApp{
				{
					Name:        fmt.Sprintf("concurrent-app-%d", index),
					Description: fmt.Sprintf("Concurrent test app %d", index),
					Linux: types.OSConfig{
						InstallMethod:  "generic",
						InstallCommand: fmt.Sprintf("echo 'concurrent install %d'", index),
					},
				},
			}

			settings := config.CrossPlatformSettings{}
			results[index] = installer.InstallApps(ctx, apps, settings)
		}(i)
	}

	wg.Wait()

	// All installations should succeed
	for i, err := range results {
		assert.NoError(t, err, "Installation %d should succeed", i)
	}
}

func TestStreamingInstaller_LargeNumberOfApps(t *testing.T) {
	ctx := context.Background()
	mockRepo := &MockRepository{}
	mockExecutor := NewMockCommandExecutor()
	installer := NewStreamingInstallerWithExecutor(nil, mockRepo, ctx, mockExecutor, getTestSettings())

	// Create many apps
	const numApps = 50
	apps := make([]types.CrossPlatformApp, numApps)
	for i := 0; i < numApps; i++ {
		apps[i] = types.CrossPlatformApp{
			Name:        fmt.Sprintf("bulk-app-%d", i),
			Description: fmt.Sprintf("Bulk test app %d", i),
			Linux: types.OSConfig{
				InstallMethod:  "generic",
				InstallCommand: fmt.Sprintf("echo 'bulk install %d'", i),
			},
		}
	}

	settings := config.CrossPlatformSettings{}

	// Should handle large number of apps without issues
	start := time.Now()
	err := installer.InstallApps(context.Background(), apps, settings)
	duration := time.Since(start)

	assert.NoError(t, err)

	// Shouldn't take too long for echo commands
	assert.Less(t, duration, 30*time.Second)

	// Verify all apps were installed
	installedApps, err := mockRepo.GetApps()
	require.NoError(t, err)
	assert.Len(t, installedApps, numApps)
}

// Benchmark tests for integration scenarios

func BenchmarkStreamingInstaller_SingleApp(b *testing.B) {
	ctx := context.Background()

	apps := []types.CrossPlatformApp{
		{
			Name:        "benchmark-app",
			Description: "Benchmark test app",
			Linux: types.OSConfig{
				InstallMethod:  "generic",
				InstallCommand: "echo 'benchmark'",
			},
		},
	}

	settings := config.CrossPlatformSettings{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mockRepo := &MockRepository{}
		mockExecutor := NewMockCommandExecutor()
		installer := NewStreamingInstallerWithExecutor(nil, mockRepo, ctx, mockExecutor, getTestSettings())
		_ = installer.InstallApps(context.Background(), apps, settings)
	}
}

func BenchmarkStreamingInstaller_MultipleApps(b *testing.B) {
	ctx := context.Background()

	apps := make([]types.CrossPlatformApp, 10)
	for i := 0; i < 10; i++ {
		apps[i] = types.CrossPlatformApp{
			Name:        fmt.Sprintf("benchmark-app-%d", i),
			Description: fmt.Sprintf("Benchmark test app %d", i),
			Linux: types.OSConfig{
				InstallMethod:  "generic",
				InstallCommand: fmt.Sprintf("echo 'benchmark %d'", i),
			},
		}
	}

	settings := config.CrossPlatformSettings{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mockRepo := &MockRepository{}
		mockExecutor := NewMockCommandExecutor()
		installer := NewStreamingInstallerWithExecutor(nil, mockRepo, ctx, mockExecutor, getTestSettings())
		_ = installer.InstallApps(context.Background(), apps, settings)
	}
}
