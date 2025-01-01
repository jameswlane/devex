package fs

import (
	"bytes"
	"io"
	"os"
)

type MockFS struct {
	Files  map[string]*bytes.Buffer
	Dirs   map[string]bool
	Errors map[string]error
}

func NewMockFS() *MockFS {
	return &MockFS{
		Files:  make(map[string]*bytes.Buffer),
		Dirs:   make(map[string]bool),
		Errors: make(map[string]error),
	}
}

func (m *MockFS) ReadFile(name string) ([]byte, error) {
	if err, exists := m.Errors[name]; exists {
		return nil, err
	}
	if file, ok := m.Files[name]; ok {
		return file.Bytes(), nil
	}
	return nil, os.ErrNotExist
}

func (m *MockFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	if err, exists := m.Errors[name]; exists {
		return err
	}
	m.Files[name] = bytes.NewBuffer(data)
	return nil
}

func (m *MockFS) Create(name string) (io.WriteCloser, error) {
	if err, exists := m.Errors[name]; exists {
		return nil, err
	}
	buf := new(bytes.Buffer)
	m.Files[name] = buf
	return nopCloser{buf}, nil
}

func (m *MockFS) MkdirAll(path string, perm os.FileMode) error {
	if err, exists := m.Errors[path]; exists {
		return err
	}
	m.Dirs[path] = true
	return nil
}

func (m *MockFS) Remove(name string) error {
	if err, exists := m.Errors[name]; exists {
		return err
	}
	delete(m.Files, name)
	delete(m.Dirs, name)
	return nil
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error {
	return nil
}
