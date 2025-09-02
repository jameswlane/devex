package mocks

import (
	"fmt"
	"sync"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type MockRepository struct {
	data  map[string]string
	apps  map[string]*types.AppConfig
	mutex sync.Mutex
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		data: make(map[string]string),
		apps: make(map[string]*types.AppConfig),
	}
}

func (m *MockRepository) ListApps() ([]types.AppConfig, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	apps := make([]types.AppConfig, 0, len(m.apps))
	for _, app := range m.apps {
		apps = append(apps, *app)
	}
	return apps, nil
}

func (m *MockRepository) SaveApp(app types.AppConfig) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.apps[app.Name]; exists {
		return fmt.Errorf("app already exists: %s", app.Name)
	}
	m.apps[app.Name] = &app
	return nil
}

func (m *MockRepository) Set(key, value string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.data[key] = value
	return nil
}

func (m *MockRepository) Get(key string) (string, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	val, ok := m.data[key]
	if !ok {
		return "", fmt.Errorf("key not found: %s", key)
	}
	return val, nil
}

func (m *MockRepository) DeleteApp(name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.apps, name)
	return nil
}

func (m *MockRepository) AddApp(name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, exists := m.apps[name]; exists {
		return fmt.Errorf("app already exists: %s", name)
	}
	m.apps[name] = &types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: name,
		},
	}
	return nil
}

func (m *MockRepository) GetApp(name string) (*types.AppConfig, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	app, ok := m.apps[name]
	if !ok {
		return nil, fmt.Errorf("app not found")
	}
	return app, nil
}

func (m *MockRepository) GetAll() (map[string]string, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	result := make(map[string]string)
	for k, v := range m.data {
		result[k] = v
	}
	return result, nil
}

// MockSystemRepository implements types.SystemRepository for testing
type MockSystemRepository struct {
	data  map[string]string
	mutex sync.Mutex
}

func NewMockSystemRepository() *MockSystemRepository {
	return &MockSystemRepository{
		data: make(map[string]string),
	}
}

func (m *MockSystemRepository) Get(key string) (string, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	val, ok := m.data[key]
	if !ok {
		return "", fmt.Errorf("key not found: %s", key)
	}
	return val, nil
}

func (m *MockSystemRepository) Set(key, value string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.data[key] = value
	return nil
}

func (m *MockSystemRepository) GetAll() (map[string]string, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	result := make(map[string]string)
	for k, v := range m.data {
		result[k] = v
	}
	return result, nil
}

// FailingMockRepository implements Repository interface but returns errors for testing
type FailingMockRepository struct{}

func (m *FailingMockRepository) ListApps() ([]types.AppConfig, error) {
	return nil, fmt.Errorf("mock repository error")
}

func (m *FailingMockRepository) SaveApp(app types.AppConfig) error {
	return fmt.Errorf("mock repository error")
}

func (m *FailingMockRepository) Set(key, value string) error {
	return fmt.Errorf("mock repository error")
}

func (m *FailingMockRepository) Get(key string) (string, error) {
	return "", fmt.Errorf("mock repository error")
}

func (m *FailingMockRepository) GetAll() (map[string]string, error) {
	return nil, fmt.Errorf("mock repository error")
}

func (m *FailingMockRepository) DeleteApp(name string) error {
	return fmt.Errorf("mock repository error")
}

func (m *FailingMockRepository) AddApp(name string) error {
	return fmt.Errorf("mock repository error")
}

func (m *FailingMockRepository) GetApp(name string) (*types.AppConfig, error) {
	return nil, fmt.Errorf("mock repository error")
}

// FailingMockSystemRepository implements SystemRepository interface but returns errors for testing
type FailingMockSystemRepository struct{}

func (m *FailingMockSystemRepository) Get(key string) (string, error) {
	return "", fmt.Errorf("mock system repository error")
}

func (m *FailingMockSystemRepository) Set(key, value string) error {
	return fmt.Errorf("mock system repository error")
}

func (m *FailingMockSystemRepository) GetAll() (map[string]string, error) {
	return nil, fmt.Errorf("mock system repository error")
}
