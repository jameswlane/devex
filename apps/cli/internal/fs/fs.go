package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/afero"
)

var (
	AppFs    afero.Fs = afero.NewOsFs()
	AppAfero          = &afero.Afero{Fs: AppFs}
	initOnce sync.Once
)

func ensureInitialized() {
	initOnce.Do(func() {
		if AppFs == nil {
			AppFs = afero.NewOsFs()
			AppAfero = &afero.Afero{Fs: AppFs}
		}
	})
}

func UseMemMapFs() {
	SetFs(afero.NewMemMapFs())
}

func UseOsFs() {
	SetFs(afero.NewOsFs())
}

func SetFs(fs afero.Fs) {
	AppFs = fs
	AppAfero = &afero.Afero{Fs: fs}
}

// Filesystem Operations
func Chmod(name string, mode os.FileMode) error {
	ensureInitialized()
	return AppFs.Chmod(name, mode)
}

func Chown(name string, uid, gid int) error {
	ensureInitialized()
	return AppFs.Chown(name, uid, gid)
}

func Chtimes(name string, atime time.Time, mtime time.Time) error {
	ensureInitialized()
	return AppFs.Chtimes(name, atime, mtime)
}

func Create(name string) (afero.File, error) {
	ensureInitialized()
	return AppFs.Create(name)
}

func Mkdir(name string, perm os.FileMode) error {
	ensureInitialized()
	return AppFs.Mkdir(name, perm)
}

func MkdirAll(path string, perm os.FileMode) error {
	ensureInitialized()
	return AppAfero.MkdirAll(path, perm)
}

func Remove(name string) error {
	ensureInitialized()
	return AppFs.Remove(name)
}

func RemoveAll(path string) error {
	ensureInitialized()
	return AppFs.RemoveAll(path)
}

func Rename(oldname string, newname string) error {
	ensureInitialized()
	return AppFs.Rename(oldname, newname)
}

// File and Directory Information
func Stat(name string) (os.FileInfo, error) {
	ensureInitialized()
	return AppFs.Stat(name)
}

func ReadDir(dirname string) ([]os.FileInfo, error) {
	ensureInitialized()
	return afero.ReadDir(AppFs, dirname)
}

func Exists(path string) (bool, error) {
	ensureInitialized()
	return AppAfero.Exists(path)
}

func DirExists(path string) (bool, error) {
	ensureInitialized()
	return AppAfero.DirExists(path)
}

// File I/O Operations
func Open(name string) (afero.File, error) {
	ensureInitialized()
	return AppFs.Open(name)
}

func WriteFile(filename string, data []byte, perm os.FileMode) error {
	ensureInitialized()
	return AppAfero.WriteFile(filename, data, perm)
}

func ReadFile(filename string) ([]byte, error) {
	ensureInitialized()
	return AppAfero.ReadFile(filename)
}

func TempFile(dir, prefix string) (afero.File, error) {
	ensureInitialized()
	if dir == "" {
		dir = os.TempDir()
	}
	return AppAfero.TempFile(dir, prefix+"tmp")
}

// Miscellaneous
func Walk(root string, walkFn filepath.WalkFunc) error {
	ensureInitialized()
	return AppAfero.Walk(root, walkFn)
}

// Metadata
func Name() string {
	ensureInitialized()
	return AppFs.Name()
}

func GetTempDir(subPath string) string {
	ensureInitialized()
	return AppAfero.GetTempDir(subPath)
}

// Common Patterns
func FileExistsAndIsFile(path string) (bool, error) {
	exists, err := Exists(path)
	if err != nil || !exists {
		return false, err
	}
	isDir, err := DirExists(path)
	return !isDir, err
}

func EnsureDir(path string, perm os.FileMode) error {
	if exists, err := AppAfero.DirExists(path); err != nil || exists {
		return err
	}
	return AppAfero.MkdirAll(path, perm)
}

func ReadFileIfExists(path string) ([]byte, error) {
	exists, err := Exists(path)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return ReadFile(path)
}

// MockFsWithErrors is a mock filesystem that can simulate errors for testing.
type MockFsWithErrors struct {
	afero.MemMapFs
	FailMkdirAll bool
	FailCreate   bool
	FailRemove   bool
	FailStat     bool
}

// MkdirAll mocks the creation of directories, optionally failing.
func (m *MockFsWithErrors) MkdirAll(path string, perm os.FileMode) error {
	if m.FailMkdirAll {
		return errors.New("mock directory creation failure")
	}
	return m.MemMapFs.MkdirAll(path, perm)
}

// Create mocks file creation, optionally failing.
func (m *MockFsWithErrors) Create(name string) (afero.File, error) {
	if m.FailCreate {
		fmt.Println("MockFsWithErrors: Failing Create operation")
		return nil, errors.New("mock failure in Create")
	}
	fmt.Println("MockFsWithErrors: Creating file successfully")
	return m.MemMapFs.Create(name)
}

// Remove mocks file removal, optionally failing.
func (m *MockFsWithErrors) Remove(name string) error {
	if m.FailRemove {
		return errors.New("mock failure in Remove")
	}
	return m.MemMapFs.Remove(name)
}

// Stat mocks file status checking, optionally failing.
func (m *MockFsWithErrors) Stat(name string) (os.FileInfo, error) {
	if m.FailStat {
		return nil, errors.New("mock failure in Stat")
	}
	return m.MemMapFs.Stat(name)
}
