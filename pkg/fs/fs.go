package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/jameswlane/devex/pkg/log"

	"github.com/spf13/afero"
)

var (
	AppFs    afero.Fs = afero.NewOsFs()
	AppAfero          = &afero.Afero{Fs: AppFs}
)

// Ensure that AppFs is initialized before any operation
func ensureInitialized() {
	if AppFs == nil {
		panic("AppFs is not initialized. Set AppFs to a valid Afero filesystem backend.")
	}
}

func UseMemMapFs() {
	AppFs = afero.NewMemMapFs()
	AppAfero = &afero.Afero{Fs: AppFs}
}

func UseOsFs() {
	AppFs = afero.NewOsFs()
	AppAfero = &afero.Afero{Fs: AppFs}
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
	return AppFs.MkdirAll(path, perm)
}

func Remove(name string) error {
	ensureInitialized()
	err := AppFs.Remove(name)
	if err != nil {
		log.Error("Remove failed", "name", name, "error", err)
		return fmt.Errorf("failed to remove file or directory '%s': %w", name, err)
	}
	log.Info("Remove succeeded", "name", name)
	return nil
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

func IsDir(path string) (bool, error) {
	ensureInitialized()
	return afero.IsDir(AppFs, path)
}

func IsEmpty(path string) (bool, error) {
	ensureInitialized()
	return afero.IsEmpty(AppFs, path)
}

func Exists(path string) (bool, error) {
	ensureInitialized()
	return afero.Exists(AppFs, path)
}

func DirExists(path string) (bool, error) {
	ensureInitialized()
	return afero.DirExists(AppFs, path)
}

// File I/O Operations
func Open(name string) (afero.File, error) {
	ensureInitialized()
	return AppFs.Open(name)
}

func OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	ensureInitialized()
	return AppFs.OpenFile(name, flag, perm)
}

func WriteFile(filename string, data []byte, perm os.FileMode) error {
	ensureInitialized()
	err := AppAfero.WriteFile(filename, data, perm)
	if err != nil {
		log.Error("WriteFile failed", "filename", filename, "error", err)
		return fmt.Errorf("failed to write file '%s': %w", filename, err)
	}
	log.Info("WriteFile succeeded", "filename", filename)
	return nil
}

func ReadFile(filename string) ([]byte, error) {
	ensureInitialized()
	return AppAfero.ReadFile(filename)
}

func FileContainsBytes(filename string, subslice []byte) (bool, error) {
	ensureInitialized()
	return afero.FileContainsBytes(AppFs, filename, subslice)
}

func WriteReader(path string, r io.Reader) error {
	ensureInitialized()
	return afero.WriteReader(AppFs, path, r)
}

func SafeWriteReader(path string, r io.Reader) error {
	ensureInitialized()
	return afero.SafeWriteReader(AppFs, path, r)
}

// Temporary Files and Directories
func TempDir(dir, prefix string) (string, error) {
	ensureInitialized()
	return afero.TempDir(AppFs, dir, prefix)
}

func TempFile(dir, prefix string) (afero.File, error) {
	ensureInitialized()
	return afero.TempFile(AppFs, dir, prefix)
}

// Miscellaneous
func Walk(root string, walkFn filepath.WalkFunc) error {
	ensureInitialized()
	return afero.Walk(AppFs, root, walkFn)
}

// Metadata
func Name() string {
	ensureInitialized()
	return AppFs.Name()
}

func GetTempDir(subPath string) string {
	ensureInitialized()
	return afero.GetTempDir(AppFs, subPath)
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
	exists, err := DirExists(path)
	if err != nil {
		log.Error("EnsureDir failed to check directory existence", "path", path, "error", err)
		return fmt.Errorf("failed to check directory existence '%s': %w", path, err)
	}
	if !exists {
		log.Info("EnsureDir creating directory", "path", path)
		return MkdirAll(path, perm)
	}
	log.Info("EnsureDir directory already exists", "path", path)
	return nil
}

func ReadFileIfExists(path string) ([]byte, error) {
	exists, err := Exists(path)
	if err != nil {
		log.Error("ReadFileIfExists failed to check existence", "path", path, "error", err)
		return nil, err
	}
	if !exists {
		log.Info("ReadFileIfExists file does not exist", "path", path)
		return nil, nil // Return nil if file doesn't exist
	}
	return ReadFile(path)
}

func WriteStringToFile(path, content string, perm os.FileMode) error {
	ensureInitialized()
	file, err := AppFs.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		log.Error("WriteStringToFile failed to open file", "path", path, "error", err)
		return fmt.Errorf("failed to open file '%s': %w", path, err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			log.Warn("WriteStringToFile failed to close file", "path", path, "error", cerr)
		}
	}()

	_, err = file.WriteString(content)
	if err != nil {
		log.Error("WriteStringToFile failed to write content", "path", path, "error", err)
		return fmt.Errorf("failed to write content to file '%s': %w", path, err)
	}

	log.Info("WriteStringToFile succeeded", "path", path)
	return nil
}
