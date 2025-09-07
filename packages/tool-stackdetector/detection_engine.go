package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Technology represents a detected technology with its metadata
type Technology struct {
	Name        string
	Category    string
	Confidence  int // 1-10 scale
	Description string
	Files       []string
}

// statCache provides thread-safe caching for os.Stat() calls to improve performance
type statCache struct {
	cache map[string]statResult
	mutex sync.RWMutex
}

// statResult represents a cached stat result
type statResult struct {
	info  os.FileInfo
	err   error
	isDir bool
}

// newStatCache creates a new stat cache instance
func newStatCache() *statCache {
	return &statCache{
		cache: make(map[string]statResult),
	}
}

// stat performs a cached os.Stat() call
func (sc *statCache) stat(path string) (os.FileInfo, error) {
	sc.mutex.RLock()
	if result, exists := sc.cache[path]; exists {
		sc.mutex.RUnlock()
		return result.info, result.err
	}
	sc.mutex.RUnlock()

	// Perform actual stat call
	info, err := os.Stat(path)

	// Cache the result
	sc.mutex.Lock()
	sc.cache[path] = statResult{
		info:  info,
		err:   err,
		isDir: info != nil && info.IsDir(),
	}
	sc.mutex.Unlock()

	return info, err
}

// fileExists checks if a file exists using cached stat results
func (sc *statCache) fileExists(path string) bool {
	_, err := sc.stat(path)
	return err == nil
}

// isDir checks if a path is a directory using cached stat results
func (sc *statCache) isDir(path string) bool {
	sc.mutex.RLock()
	if result, exists := sc.cache[path]; exists {
		sc.mutex.RUnlock()
		return result.isDir
	}
	sc.mutex.RUnlock()

	info, err := sc.stat(path)
	return err == nil && info.IsDir()
}

// DetectStack performs comprehensive stack detection in the given directory
func (p *StackDetectorPlugin) DetectStack(dir string) []Technology {
	var detected []Technology

	// Create a stat cache for this detection session
	cache := newStatCache()

	// File-based detection
	fileDetections := p.detectByFiles(dir, cache)
	detected = append(detected, fileDetections...)

	// Directory-based detection
	dirDetections := p.detectByDirectories(dir, cache)
	detected = append(detected, dirDetections...)

	// Content-based detection (for more specific technologies)
	contentDetections := p.detectByContent(dir, cache)
	detected = append(detected, contentDetections...)

	// Remove duplicates and sort by confidence
	detected = p.deduplicateAndSort(detected)

	return detected
}

// detectByFiles detects technologies based on the presence of specific files
func (p *StackDetectorPlugin) detectByFiles(dir string, cache *statCache) []Technology {
	var detected []Technology

	// Enhanced file detectors with categories and confidence levels
	fileDetectors := map[string]Technology{
		"package.json": {
			Name:        "Node.js",
			Category:    "Runtime",
			Confidence:  9,
			Description: "JavaScript/Node.js project with npm dependencies",
		},
		"requirements.txt": {
			Name:        "Python",
			Category:    "Language",
			Confidence:  8,
			Description: "Python project with pip dependencies",
		},
		"Pipfile": {
			Name:        "Python (Pipenv)",
			Category:    "Package Manager",
			Confidence:  9,
			Description: "Python project using Pipenv for dependency management",
		},
		"pyproject.toml": {
			Name:        "Python (Poetry)",
			Category:    "Package Manager",
			Confidence:  9,
			Description: "Python project using Poetry for dependency management",
		},
		"Cargo.toml": {
			Name:        "Rust",
			Category:    "Language",
			Confidence:  10,
			Description: "Rust project with Cargo package manager",
		},
		"go.mod": {
			Name:        "Go",
			Category:    "Language",
			Confidence:  10,
			Description: "Go project with module dependencies",
		},
		"composer.json": {
			Name:        "PHP",
			Category:    "Language",
			Confidence:  9,
			Description: "PHP project with Composer dependencies",
		},
		"pom.xml": {
			Name:        "Java (Maven)",
			Category:    "Build System",
			Confidence:  9,
			Description: "Java project using Maven build system",
		},
		"build.gradle": {
			Name:        "Java/Kotlin (Gradle)",
			Category:    "Build System",
			Confidence:  9,
			Description: "Java or Kotlin project using Gradle build system",
		},
		"Gemfile": {
			Name:        "Ruby",
			Category:    "Language",
			Confidence:  9,
			Description: "Ruby project with Bundler dependencies",
		},
		"mix.exs": {
			Name:        "Elixir",
			Category:    "Language",
			Confidence:  10,
			Description: "Elixir project with Mix build tool",
		},
		"pubspec.yaml": {
			Name:        "Dart/Flutter",
			Category:    "Framework",
			Confidence:  10,
			Description: "Dart or Flutter project",
		},
		"Dockerfile": {
			Name:        "Docker",
			Category:    "Containerization",
			Confidence:  8,
			Description: "Containerized application using Docker",
		},
		"docker-compose.yml": {
			Name:        "Docker Compose",
			Category:    "Orchestration",
			Confidence:  8,
			Description: "Multi-container application using Docker Compose",
		},
		"docker-compose.yaml": {
			Name:        "Docker Compose",
			Category:    "Orchestration",
			Confidence:  8,
			Description: "Multi-container application using Docker Compose",
		},
		"yarn.lock": {
			Name:        "Node.js (Yarn)",
			Category:    "Package Manager",
			Confidence:  7,
			Description: "Node.js project using Yarn package manager",
		},
		"package-lock.json": {
			Name:        "Node.js (npm)",
			Category:    "Package Manager",
			Confidence:  7,
			Description: "Node.js project using npm package manager",
		},
		"tsconfig.json": {
			Name:        "TypeScript",
			Category:    "Language",
			Confidence:  9,
			Description: "TypeScript project with custom configuration",
		},
		"Makefile": {
			Name:        "Make",
			Category:    "Build System",
			Confidence:  6,
			Description: "Project using Make build system",
		},
		"CMakeLists.txt": {
			Name:        "CMake",
			Category:    "Build System",
			Confidence:  9,
			Description: "C/C++ project using CMake build system",
		},
	}

	// Check for files
	for file, tech := range fileDetectors {
		path := filepath.Join(dir, file)
		if cache.fileExists(path) {
			tech.Files = []string{file}
			detected = append(detected, tech)
		}
	}

	return detected
}

// detectByDirectories detects technologies based on the presence of specific directories
func (p *StackDetectorPlugin) detectByDirectories(dir string, cache *statCache) []Technology {
	var detected []Technology

	dirDetectors := map[string]Technology{
		"node_modules": {
			Name:        "Node.js Dependencies",
			Category:    "Dependencies",
			Confidence:  8,
			Description: "Node.js project with installed dependencies",
		},
		".git": {
			Name:        "Git",
			Category:    "Version Control",
			Confidence:  10,
			Description: "Git version control system",
		},
		"venv": {
			Name:        "Python (venv)",
			Category:    "Virtual Environment",
			Confidence:  7,
			Description: "Python virtual environment",
		},
		"env": {
			Name:        "Python (virtualenv)",
			Category:    "Virtual Environment",
			Confidence:  6,
			Description: "Python virtual environment",
		},
		"target": {
			Name:        "Build Artifacts",
			Category:    "Build Output",
			Confidence:  5,
			Description: "Compiled build artifacts (Rust/Java/Scala)",
		},
		"dist": {
			Name:        "Distribution Build",
			Category:    "Build Output",
			Confidence:  5,
			Description: "Distribution build output",
		},
		".next": {
			Name:        "Next.js",
			Category:    "Framework",
			Confidence:  9,
			Description: "Next.js React framework",
		},
		".nuxt": {
			Name:        "Nuxt.js",
			Category:    "Framework",
			Confidence:  9,
			Description: "Nuxt.js Vue framework",
		},
		"vendor": {
			Name:        "Vendor Dependencies",
			Category:    "Dependencies",
			Confidence:  6,
			Description: "Vendor/third-party dependencies",
		},
	}

	for dirName, tech := range dirDetectors {
		path := filepath.Join(dir, dirName)
		if cache.isDir(path) {
			tech.Files = []string{dirName + "/"}
			detected = append(detected, tech)
		}
	}

	return detected
}

// detectByContent performs content-based detection for more specific identification
func (p *StackDetectorPlugin) detectByContent(dir string, cache *statCache) []Technology {
	var detected []Technology

	// Check package.json for specific frameworks
	if packageTech := p.detectFromPackageJson(dir, cache); packageTech != nil {
		detected = append(detected, *packageTech)
	}

	// Check for specific configuration patterns
	detected = append(detected, p.detectFrameworkConfigs(dir, cache)...)

	return detected
}

// detectFromPackageJson analyzes package.json content for framework detection
func (p *StackDetectorPlugin) detectFromPackageJson(dir string, cache *statCache) *Technology {
	packagePath := filepath.Join(dir, "package.json")
	if !cache.fileExists(packagePath) {
		return nil
	}

	content, err := os.ReadFile(packagePath)
	if err != nil {
		return nil
	}

	contentStr := string(content)

	// Look for popular frameworks in dependencies
	frameworks := map[string]Technology{
		"react": {
			Name:        "React",
			Category:    "Framework",
			Confidence:  9,
			Description: "React JavaScript library for building user interfaces",
		},
		"vue": {
			Name:        "Vue.js",
			Category:    "Framework",
			Confidence:  9,
			Description: "Vue.js progressive JavaScript framework",
		},
		"angular": {
			Name:        "Angular",
			Category:    "Framework",
			Confidence:  9,
			Description: "Angular TypeScript framework",
		},
		"express": {
			Name:        "Express.js",
			Category:    "Framework",
			Confidence:  8,
			Description: "Express.js Node.js web framework",
		},
		"next": {
			Name:        "Next.js",
			Category:    "Framework",
			Confidence:  9,
			Description: "Next.js React production framework",
		},
	}

	for dep, tech := range frameworks {
		if strings.Contains(contentStr, "\""+dep+"\"") {
			tech.Files = []string{"package.json"}
			return &tech
		}
	}

	return nil
}

// detectFrameworkConfigs detects specific framework configuration files
func (p *StackDetectorPlugin) detectFrameworkConfigs(dir string, cache *statCache) []Technology {
	var detected []Technology

	configDetectors := map[string]Technology{
		"webpack.config.js": {
			Name:        "Webpack",
			Category:    "Build Tool",
			Confidence:  8,
			Description: "Webpack module bundler",
		},
		"vite.config.js": {
			Name:        "Vite",
			Category:    "Build Tool",
			Confidence:  9,
			Description: "Vite build tool",
		},
		"rollup.config.js": {
			Name:        "Rollup",
			Category:    "Build Tool",
			Confidence:  8,
			Description: "Rollup module bundler",
		},
		"tailwind.config.js": {
			Name:        "Tailwind CSS",
			Category:    "CSS Framework",
			Confidence:  8,
			Description: "Tailwind CSS utility-first framework",
		},
		".eslintrc": {
			Name:        "ESLint",
			Category:    "Linting",
			Confidence:  7,
			Description: "ESLint JavaScript linter",
		},
		".prettierrc": {
			Name:        "Prettier",
			Category:    "Code Formatting",
			Confidence:  7,
			Description: "Prettier code formatter",
		},
	}

	for configFile, tech := range configDetectors {
		if p.configFileExists(dir, configFile, cache) {
			tech.Files = []string{configFile}
			detected = append(detected, tech)
		}
	}

	return detected
}

// configFileExists checks if a configuration file exists with various extensions
func (p *StackDetectorPlugin) configFileExists(dir, baseFile string, cache *statCache) bool {
	extensions := []string{"", ".js", ".ts", ".json", ".yaml", ".yml"}

	for _, ext := range extensions {
		path := filepath.Join(dir, baseFile+ext)
		if cache.fileExists(path) {
			return true
		}
	}
	return false
}

// deduplicateAndSort removes duplicates and sorts technologies by confidence
func (p *StackDetectorPlugin) deduplicateAndSort(technologies []Technology) []Technology {
	seen := make(map[string]bool)
	var result []Technology

	for _, tech := range technologies {
		if !seen[tech.Name] {
			seen[tech.Name] = true
			result = append(result, tech)
		}
	}

	// Sort by confidence (highest first)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Confidence < result[j].Confidence {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}
