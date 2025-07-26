package mocks

import (
	"fmt"
	"sync"

	"github.com/jameswlane/devex/pkg/types"
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
