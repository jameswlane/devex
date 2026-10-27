package sdk

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// MaxPluginBinarySize caps how many bytes are extracted for a single plugin executable
const MaxPluginBinarySize int64 = 512 * 1024 * 1024

// PayloadFormat describes how a downloaded plugin payload is packaged
type PayloadFormat string

const (
	PayloadFormatBinary PayloadFormat = "binary"
	PayloadFormatTarGz  PayloadFormat = "tar.gz"
	PayloadFormatZip    PayloadFormat = "zip"
)

var (
	gzipMagic = []byte{0x1f, 0x8b}
	zipMagic  = []byte("PK\x03\x04")
)

// ErrPluginNotInArchive is returned when an archive does not contain the expected plugin executable
var ErrPluginNotInArchive = errors.New("plugin executable not found in archive")

// pluginExecutableName returns the executable file name for a plugin on the given OS
func pluginExecutableName(pluginName, goos string) string {
	name := "devex-plugin-" + pluginName
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// DetectPayloadFormat sniffs the first bytes of a file to determine how it is packaged
func DetectPayloadFormat(filePath string) (PayloadFormat, error) {
	file, err := os.Open(filePath) // #nosec G304 - path is created by the downloader
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	header := make([]byte, 4)
	n, err := io.ReadFull(file, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", err
	}
	header = header[:n]

	switch {
	case bytes.HasPrefix(header, gzipMagic):
		return PayloadFormatTarGz, nil
	case bytes.HasPrefix(header, zipMagic):
		return PayloadFormatZip, nil
	default:
		return PayloadFormatBinary, nil
	}
}

// ExtractPluginExecutable writes the plugin executable contained in payloadPath to destPath.
// Raw binaries are copied as-is; tar.gz and zip archives are searched for the single entry
// whose base name matches the plugin executable name. Entry paths are never used to build
// the destination path, so archive contents cannot escape destPath.
func ExtractPluginExecutable(payloadPath, destPath, pluginName, goos string) (PayloadFormat, error) {
	format, err := DetectPayloadFormat(payloadPath)
	if err != nil {
		return "", fmt.Errorf("failed to inspect plugin payload: %w", err)
	}

	want := pluginExecutableName(pluginName, goos)

	switch format {
	case PayloadFormatTarGz:
		err = extractFromTarGz(payloadPath, destPath, want)
	case PayloadFormatZip:
		err = extractFromZip(payloadPath, destPath, want)
	default:
		err = copyLimited(payloadPath, destPath)
	}
	if err != nil {
		return format, err
	}
	return format, nil
}

func extractFromTarGz(archivePath, destPath, want string) error {
	file, err := os.Open(archivePath) // #nosec G304 - path is created by the downloader
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	gz, err := gzip.NewReader(bufio.NewReader(file))
	if err != nil {
		return fmt.Errorf("invalid gzip archive: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("%w: %s", ErrPluginNotInArchive, want)
		}
		if err != nil {
			return fmt.Errorf("invalid tar archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || path.Base(hdr.Name) != want {
			continue
		}
		if hdr.Size > MaxPluginBinarySize {
			return fmt.Errorf("plugin executable %s exceeds %d bytes", want, MaxPluginBinarySize)
		}
		return writeLimited(tr, destPath)
	}
}

func extractFromZip(archivePath, destPath, want string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("invalid zip archive: %w", err)
	}
	defer func() { _ = zr.Close() }()

	for _, entry := range zr.File {
		if !entry.Mode().IsRegular() || path.Base(entry.Name) != want {
			continue
		}
		if entry.UncompressedSize64 > uint64(MaxPluginBinarySize) {
			return fmt.Errorf("plugin executable %s exceeds %d bytes", want, MaxPluginBinarySize)
		}
		rc, err := entry.Open()
		if err != nil {
			return fmt.Errorf("failed to open %s in zip archive: %w", want, err)
		}
		err = writeLimited(rc, destPath)
		_ = rc.Close()
		return err
	}
	return fmt.Errorf("%w: %s", ErrPluginNotInArchive, want)
}

func copyLimited(srcPath, destPath string) error {
	src, err := os.Open(srcPath) // #nosec G304 - path is created by the downloader
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	return writeLimited(src, destPath)
}

// writeLimited copies at most MaxPluginBinarySize bytes into a new file at destPath
func writeLimited(r io.Reader, destPath string) error {
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) // #nosec G304 - destPath is a downloader-owned temp path
	if err != nil {
		return err
	}

	n, copyErr := io.Copy(out, io.LimitReader(r, MaxPluginBinarySize+1))
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("failed to write plugin executable: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to write plugin executable: %w", closeErr)
	}
	if n > MaxPluginBinarySize {
		return fmt.Errorf("plugin executable exceeds %d bytes", MaxPluginBinarySize)
	}
	return nil
}

// InstallRecord describes what was installed for a plugin, stored next to the executable
type InstallRecord struct {
	Plugin          string        `json:"plugin"`
	Version         string        `json:"version"`
	PayloadChecksum string        `json:"payload_checksum"`
	PayloadFormat   PayloadFormat `json:"payload_format"`
	BinaryChecksum  string        `json:"binary_checksum"`
	InstalledAt     time.Time     `json:"installed_at"`
}

// installRecordPath returns the hidden sidecar path, which plugin discovery ignores
func installRecordPath(pluginDir, pluginName string) string {
	return filepath.Join(pluginDir, ".devex-plugin-"+pluginName+".install.json")
}

func readInstallRecord(pluginDir, pluginName string) (*InstallRecord, error) {
	data, err := os.ReadFile(installRecordPath(pluginDir, pluginName))
	if err != nil {
		return nil, err
	}
	var record InstallRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func writeInstallRecord(pluginDir string, record InstallRecord) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(installRecordPath(pluginDir, record.Plugin), data, 0o600)
}

func fileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath) // #nosec G304 - path is inside the plugin directory
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// normalizeChecksum lowercases a checksum and strips an optional "sha256:" prefix
func normalizeChecksum(checksum string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(checksum)), "sha256:")
}
