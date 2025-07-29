package tui

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/types"
)

func TestStreamingInstaller_Integration(t *testing.T) {
	// Create mock repository
	mockRepo := &testMockRepository{
		apps: make(map[string]*types.AppConfig),
	}

	// Create mock settings
	mockSettings := config.CrossPlatformSettings{}

	// Create streaming installer
	installer := NewStreamingInstaller(mockRepo, mockSettings)

	// Test installing multiple apps
	apps := []string{"test-echo", "test-whoami"}
	ctx := context.Background()

	err := installer.InstallApps(ctx, apps)
	require.NoError(t, err)

	// Verify apps are tracked
	installedApps, err := installer.GetInstalledApps()
	require.NoError(t, err)

	assert.Contains(t, installedApps, "test-echo")
	assert.Contains(t, installedApps, "test-whoami")
}

func TestStreamingInstaller_CommandValidationIntegration(t *testing.T) {
	mockRepo := &testMockRepository{
		apps: make(map[string]*types.AppConfig),
	}
	mockSettings := config.CrossPlatformSettings{}
	installer := NewStreamingInstaller(mockRepo, mockSettings)

	// Test with valid commands
	apps := []string{"valid-app"}
	ctx := context.Background()

	err := installer.InstallApps(ctx, apps)
	assert.NoError(t, err)
}

func TestStreamingInstaller_ContextCancellationIntegration(t *testing.T) {
	mockRepo := &testMockRepository{
		apps: make(map[string]*types.AppConfig),
	}
	mockSettings := config.CrossPlatformSettings{}
	installer := NewStreamingInstaller(mockRepo, mockSettings)

	// Create a context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	apps := []string{"test-app"}
	err := installer.InstallApps(ctx, apps)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestStreamingInstaller_RepositoryError(t *testing.T) {
	// Create mock repository that fails on AddApp
	mockRepo := &testMockRepository{
		apps:      make(map[string]*types.AppConfig),
		shouldErr: true,
	}
	mockSettings := config.CrossPlatformSettings{}
	installer := NewStreamingInstaller(mockRepo, mockSettings)

	apps := []string{"test-app"}
	ctx := context.Background()

	err := installer.InstallApps(ctx, apps)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to track app")
}

func TestStreamingInstaller_PrePostInstallCommands(t *testing.T) {
	mockRepo := &testMockRepository{
		apps: make(map[string]*types.AppConfig),
	}
	mockSettings := config.CrossPlatformSettings{}
	installer := NewStreamingInstaller(mockRepo, mockSettings)

	// Test app with complex pre/post install commands
	apps := []string{"complex-app"}
	ctx := context.Background()

	err := installer.InstallApps(ctx, apps)
	require.NoError(t, err)

	// Verify app is tracked
	installedApps, err := installer.GetInstalledApps()
	require.NoError(t, err)

	assert.Contains(t, installedApps, "complex-app")
}

func TestStreamingInstaller_SleepCommand(t *testing.T) {
	mockRepo := &testMockRepository{
		apps: make(map[string]*types.AppConfig),
	}
	mockSettings := config.CrossPlatformSettings{}
	installer := NewStreamingInstaller(mockRepo, mockSettings)

	// Test app that sleeps
	apps := []string{"sleep-app"}
	ctx := context.Background()

	start := time.Now()
	err := installer.InstallApps(ctx, apps)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, duration, time.Second)

	// Verify app is tracked
	installedApps, err := installer.GetInstalledApps()
	require.NoError(t, err)

	assert.Contains(t, installedApps, "sleep-app")
}

func TestStreamingInstaller_SleepCancellation(t *testing.T) {
	mockRepo := &testMockRepository{
		apps: make(map[string]*types.AppConfig),
	}
	mockSettings := config.CrossPlatformSettings{}
	installer := NewStreamingInstaller(mockRepo, mockSettings)

	// Create a context that cancels after 500ms
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Test app that sleeps for longer than the timeout
	apps := []string{"sleep-5-app"}

	err := installer.InstallApps(ctx, apps)

	assert.Error(t, err)
	// Check that the error is caused by context deadline exceeded
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestStreamingInstaller_MultipleAppsWithErrors(t *testing.T) {
	mockRepo := &testMockRepository{
		apps: make(map[string]*types.AppConfig),
	}
	mockSettings := config.CrossPlatformSettings{}
	installer := NewStreamingInstaller(mockRepo, mockSettings)

	// Mix of good and bad apps
	apps := []string{"good-app-1", "bad-app", "good-app-2"}
	ctx := context.Background()

	err := installer.InstallApps(ctx, apps)

	// Should fail on the bad app
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad-app")

	// Check that good-app-1 was installed before the failure
	installedApps, err := installer.GetInstalledApps()
	require.NoError(t, err)

	assert.Contains(t, installedApps, "good-app-1")
	assert.NotContains(t, installedApps, "bad-app")
	assert.NotContains(t, installedApps, "good-app-2") // Should not be installed due to early failure
}

func TestStartInstallation_Integration(t *testing.T) {
	mockRepo := &testMockRepository{
		apps: make(map[string]*types.AppConfig),
	}
	mockSettings := config.CrossPlatformSettings{}
	installer := NewStreamingInstaller(mockRepo, mockSettings)

	// Basic integration test
	apps := []string{"simple-app"}
	ctx := context.Background()

	err := installer.InstallApps(ctx, apps)
	assert.NoError(t, err)
}

func TestStreamingInstaller_ConcurrentInstallations(t *testing.T) {
	mockRepo := &testMockRepository{
		apps: make(map[string]*types.AppConfig),
	}
	mockSettings := config.CrossPlatformSettings{}
	installer := NewStreamingInstaller(mockRepo, mockSettings)

	// Test that the installer doesn't have race conditions
	apps := []string{"concurrent-app-1", "concurrent-app-2"}
	ctx := context.Background()

	err := installer.InstallApps(ctx, apps)
	assert.NoError(t, err)
}

func TestStreamingInstaller_LargeNumberOfApps(t *testing.T) {
	mockRepo := &testMockRepository{
		apps: make(map[string]*types.AppConfig),
	}
	mockSettings := config.CrossPlatformSettings{}
	installer := NewStreamingInstaller(mockRepo, mockSettings)

	// Generate 50 test apps
	var apps []string
	for i := 0; i < 50; i++ {
		apps = append(apps, fmt.Sprintf("app-%d", i))
	}

	ctx := context.Background()
	err := installer.InstallApps(ctx, apps)
	require.NoError(t, err)

	// Verify all apps are tracked
	installedApps, err := installer.GetInstalledApps()
	require.NoError(t, err)

	assert.Len(t, installedApps, 50)
}

// testMockRepository implements types.Repository interface for testing
type testMockRepository struct {
	apps      map[string]*types.AppConfig
	shouldErr bool
	mutex     sync.Mutex
}

func (m *testMockRepository) AddApp(name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldErr {
		return fmt.Errorf("mock repository error")
	}

	m.apps[name] = &types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: name,
		},
	}
	return nil
}

func (m *testMockRepository) DeleteApp(name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldErr {
		return fmt.Errorf("mock repository error")
	}

	delete(m.apps, name)
	return nil
}

func (m *testMockRepository) GetApp(name string) (*types.AppConfig, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldErr {
		return nil, fmt.Errorf("mock repository error")
	}

	app, exists := m.apps[name]
	if !exists {
		return nil, fmt.Errorf("app not found: %s", name)
	}
	return app, nil
}

func (m *testMockRepository) ListApps() ([]types.AppConfig, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldErr {
		return nil, fmt.Errorf("mock repository error")
	}

	apps := make([]types.AppConfig, 0, len(m.apps))
	for _, app := range m.apps {
		apps = append(apps, *app)
	}
	return apps, nil
}

func (m *testMockRepository) ListAllApps() ([]types.AppConfig, error) {
	return m.ListApps()
}

func (m *testMockRepository) SaveApp(app types.AppConfig) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldErr {
		return fmt.Errorf("mock repository error")
	}

	m.apps[app.Name] = &app
	return nil
}

func (m *testMockRepository) Set(key, value string) error {
	if m.shouldErr {
		return fmt.Errorf("mock repository error")
	}
	return nil
}

func (m *testMockRepository) Get(key string) (string, error) {
	if m.shouldErr {
		return "", fmt.Errorf("mock repository error")
	}
	return "test-value", nil
}
