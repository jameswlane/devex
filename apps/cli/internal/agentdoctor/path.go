package agentdoctor

import (
	"os"
	"path/filepath"
	"strings"
)

// PathMatch describes one executable match discovered by scanning PATH.
type PathMatch struct {
	Path        string `json:"path"`
	RealPath    string `json:"real_path"`
	IsNPMGlobal bool   `json:"is_npm_global"`
}

// FindBinariesOnPATH scans every entry in probe.Getenv("PATH") in order and
// returns all distinct executable matches for binary. It does not use
// exec.LookPath so duplicate and shadowed installs are always surfaced.
func FindBinariesOnPATH(probe Probe, binary string) []PathMatch {
	probe = probe.withDefaults()
	pathEnv := probe.Getenv("PATH")
	if pathEnv == "" || binary == "" {
		return nil
	}
	sep := ":"
	if probe.GOOS == "windows" {
		sep = ";"
	}
	dirs := strings.Split(pathEnv, sep)
	seenDirs := make(map[string]bool, len(dirs))
	seenReal := make(map[string]bool, len(dirs))
	matches := make([]PathMatch, 0, 2)

	for _, dir := range dirs {
		// Relative entries resolve against the working directory, which
		// exec.LookPath refuses too; never select or launch from them.
		if dir == "" || !filepath.IsAbs(dir) {
			continue
		}
		cleanDir := filepath.Clean(dir)
		if seenDirs[cleanDir] {
			continue
		}
		seenDirs[cleanDir] = true

		for _, name := range executableCandidates(binary, probe.GOOS) {
			candidate := filepath.Join(cleanDir, name)
			info, err := probe.Stat(candidate)
			if err != nil {
				continue
			}
			if !isExecutableFile(info, probe.GOOS) {
				continue
			}
			realPath := candidate
			if resolved, err := probe.EvalSymlinks(candidate); err == nil && resolved != "" {
				realPath = filepath.Clean(resolved)
			}
			if seenReal[realPath] {
				break
			}
			seenReal[realPath] = true
			matches = append(matches, PathMatch{
				Path:        candidate,
				RealPath:    realPath,
				IsNPMGlobal: isUnderNodeModules(realPath),
			})
			break
		}
	}
	return matches
}

// FindBinary returns the first executable match for binary on probe's PATH.
func FindBinary(probe Probe, binary string) (string, bool) {
	matches := FindBinariesOnPATH(probe, binary)
	if len(matches) == 0 {
		return "", false
	}
	return matches[0].Path, true
}

func executableCandidates(binary, goos string) []string {
	if goos != "windows" {
		return []string{binary}
	}
	if filepath.Ext(binary) != "" {
		return []string{binary}
	}
	return []string{binary + ".exe", binary + ".cmd", binary + ".bat", binary}
}

func isExecutableFile(info os.FileInfo, goos string) bool {
	if info == nil {
		// Minimal test fixtures may return (nil, nil) from Stat to indicate presence.
		return true
	}
	if info.IsDir() {
		return false
	}
	if goos == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

func isUnderNodeModules(realPath string) bool {
	normalized := strings.ToLower(filepath.ToSlash(realPath))
	return strings.Contains(normalized, "/node_modules/")
}
