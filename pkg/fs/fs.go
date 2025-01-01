package fs

import (
	"io"
	"os"
)

type FileSystem interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	Create(name string) (io.WriteCloser, error)
	MkdirAll(path string, perm os.FileMode) error
	Remove(name string) error
}

type OSFileSystem struct{}

func (OSFileSystem) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (OSFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (OSFileSystem) Create(name string) (io.WriteCloser, error) {
	return os.Create(name)
}

func (OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (OSFileSystem) Remove(name string) error {
	return os.Remove(name)
}

var FileSystemInstance FileSystem = OSFileSystem{}

func ReadFile(name string) ([]byte, error) {
	return FileSystemInstance.ReadFile(name)
}

func WriteFile(name string, data []byte, perm os.FileMode) error {
	return FileSystemInstance.WriteFile(name, data, perm)
}

func Create(name string) (io.WriteCloser, error) {
	return FileSystemInstance.Create(name)
}

func MkdirAll(path string, perm os.FileMode) error {
	return FileSystemInstance.MkdirAll(path, perm)
}

func Remove(name string) error {
	return FileSystemInstance.Remove(name)
}
