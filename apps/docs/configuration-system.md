# DevEx Configuration System

The DevEx configuration system has been redesigned to use a file-per-application structure for improved maintainability and easier sharing of configurations.

## Directory Structure

The new configuration system organizes files into directories that are processed in a specific order:

```
config/
├── system/           # Core system configs (git, ssh, terminal)
├── environments/     # Programming languages, fonts, shells
├── applications/     # Application configurations by category
│   ├── databases/    # Database applications
│   ├── development/  # Development tools
│   └── optional/     # Optional applications
└── desktop/          # Desktop environment configs (GNOME, KDE)
```

### Processing Order

Directories are loaded in a fixed order to ensure proper dependency resolution:

1. **system** - Core system configurations
2. **environments** - Development environments and tools
3. **applications** - Application installations
4. **desktop** - Desktop environment customizations (loaded last)

Within each directory, YAML files are processed **alphabetically**, enabling prefix-based ordering.

## File Naming Conventions

### Alphabetical Processing with Prefixes

Files are processed in alphabetical order, allowing you to control load order using prefixes:

```
applications/development/
├── 00-priority-docker.yaml      # Loads first
├── 01-essential-git.yaml        # Loads second
├── build-essential.yaml         # Loads third (alphabetical)
├── github-cli.yaml             # Loads fourth
└── zsh.yaml                    # Loads last
```

### Valid Filename Patterns

- **✅ Valid**: `config.yaml`, `app-name.yml`, `00-priority.yaml`
- **❌ Invalid**: `../config.yaml`, `config.txt`, `.hidden.yaml`

Security validation ensures filenames are safe and don't contain path traversal attempts.

## Configuration Examples

### Application Configuration

```yaml
# config/applications/development/github-cli.yaml
name: GitHub CLI
description: GitHub official command line tool
category: Development Tools
default: true
desktop_environments:
  - all
linux:
  install_method: apt
  install_command: gh
  uninstall_command: gh
  official_support: true
  alternatives:
    - install_method: snap
      install_command: gh
      uninstall_command: gh
      official_support: true
macos:
  install_method: brew
  install_command: gh
  uninstall_command: gh
  official_support: true
windows:
  install_method: winget
  install_command: GitHub.cli
  uninstall_command: GitHub.cli
  official_support: true
```

### Programming Language Configuration

```yaml
# config/environments/programming-languages/node.yaml
name: Node.js
description: Node.js runtime
category: Programming Languages
install_method: mise
install_command: node@latest
uninstall_command: node
default: false
dependencies:
  - mise
```

### System Configuration

```yaml
# config/system/git.yaml
git:
  global_config:
    - key: user.name
      value: "${GIT_USER_NAME}"
    - key: user.email
      value: "${GIT_USER_EMAIL}"
    - key: init.defaultBranch
      value: main
    - key: pull.rebase
      value: false
```

### Desktop Environment Configuration

```yaml
# config/desktop/gnome/settings.yaml
desktop_environment: gnome
settings:
  - schema: org.gnome.desktop.interface
    key: gtk-theme
    value: "Adwaita-dark"
  - schema: org.gnome.desktop.interface
    key: cursor-theme
    value: "Adwaita"
themes:
  - name: "Tokyo Night"
    type: "gtk"
    url: "https://github.com/Fausto-Korpsvart/Tokyo-Night-GTK-Theme"
```

## Migration from Monolithic Files

The system automatically migrated from the old monolithic YAML structure:

### Before (Monolithic)
```
config/
├── applications.yaml     (86KB, 2000+ lines)
├── databases.yaml
├── programming-languages.yaml
└── desktop.yaml
```

### After (File-per-Application)
```
config/
├── applications/
│   ├── development/
│   │   ├── docker.yaml
│   │   ├── git.yaml
│   │   └── vscode.yaml
│   └── databases/
│       ├── mysql.yaml
│       └── postgresql.yaml
└── environments/
    └── programming-languages/
        ├── node.yaml
        └── python.yaml
```

## Security Features

### Path Traversal Protection

The system validates all file paths and names to prevent security vulnerabilities:

- **Filename validation**: Only allows safe characters and patterns
- **Path sanitization**: Prevents `../` traversal attacks
- **Input validation**: Validates all configuration keys and values

### Resource Management

- **Memory optimization**: Efficient Viper instance management
- **Caching**: File modification time caching to avoid unnecessary reloads
- **Parallel loading**: Concurrent file processing for large configurations

### Error Handling

- **Graceful degradation**: Invalid files don't break the entire configuration
- **Detailed logging**: Clear error messages with file and line information
- **Validation reports**: Comprehensive validation with warnings and errors

## Performance Optimizations

### Parallel Loading

For directories with many files, the system uses parallel loading:

- **Sequential loading**: Used for ≤5 files to maintain strict ordering
- **Parallel loading**: Used for >5 files with ordered result merging
- **Cache optimization**: Modification time checking to skip unchanged files

### Memory Management

- **Resource cleanup**: Proper cleanup of temporary Viper instances
- **Lazy loading**: Files loaded only when needed
- **Efficient merging**: Optimized configuration merging strategies

## Validation System

### Schema Validation

The system includes comprehensive validation for different configuration types:

```go
// Example: Application validation
validator := config.NewConfigValidator(homeDir, strict)
err := validator.ValidateDirectoryStructure(configPath)
```

### Validation Rules

- **Applications**: Require `name`, `description`, and at least one platform
- **Environments**: Validate install methods and dependencies
- **System**: Basic structure and format validation
- **Desktop**: Validate desktop environment names and settings

### Error Reporting

```
Configuration validation failed with 2 error(s):
  - Required field missing in docker.yaml.name
  - Invalid install method in node.yaml.install_method: unknown_method
  
Warnings (3):
  - Unknown category in vscode.yaml.category: Custom Tools
```

## Best Practices

### File Organization

1. **Use descriptive names**: `github-cli.yaml` not `gh.yaml`
2. **Group by category**: Keep similar apps in the same subdirectory
3. **Use prefixes for ordering**: `00-priority-`, `01-essential-`, etc.
4. **Keep files focused**: One application per file

### Configuration Structure

1. **Include all required fields**: `name`, `description`, platform configs
2. **Use official install methods**: Prefer `apt`, `brew`, `winget` over manual
3. **Add alternatives**: Provide multiple installation options when possible
4. **Document dependencies**: List required packages or tools

### Performance Considerations

1. **Optimize file size**: Keep individual files reasonably small
2. **Use caching**: Leverage built-in modification time caching
3. **Validate regularly**: Run validation to catch issues early
4. **Monitor loading time**: Large numbers of files may need optimization

## Troubleshooting

### Common Issues

**Configuration not loading:**
- Check file permissions (should be readable)
- Validate YAML syntax using `devex config validate`
- Ensure filename follows naming conventions

**Slow loading:**
- Check for very large configuration files
- Use `devex config show --format=debug` for timing information
- Consider splitting large files into smaller ones

**Validation errors:**
- Run `devex config validate --strict` for detailed reports
- Check required fields are present
- Verify install methods are supported on target platforms

### Debug Commands

```bash
# Validate configuration
devex config validate

# Show configuration status
devex config show

# List all available applications
devex list available

# Test specific category loading
devex list available --category "Development Tools"
```

## API Reference

### Configuration Functions

```go
// Load directory-based configuration
func loadDirectoryConfigs(configPath string) error

// Validate configuration structure
func ValidateDirectoryBasedConfig(homeDir string, strict bool) error

// Get configuration validator
validator := config.NewConfigValidator(homeDir, strict)
```

### Security Functions

```go
// Validate filename safety
func isValidFilename(filename string) bool

// Sanitize filenames for keys
func sanitizeFilenameForKey(filename string) string

// Check path traversal attempts
func isValidConfigPath(path string) bool
```

### Cache Management

```go
// Check if file needs reloading
shouldReload, err := globalConfigCache.shouldReloadFile(filePath)

// Clear cache (useful for testing)
globalConfigCache.clearCache()
```

This new configuration system provides a robust, secure, and performant foundation for managing DevEx configurations with improved maintainability and user experience.
