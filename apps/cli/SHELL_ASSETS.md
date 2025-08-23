# Shell Assets Documentation

This document describes the standardized structure for shell configuration assets in the DevEx CLI.

## Directory Structure

```
assets/
├── bash/
│   ├── bashrc              → ~/.bashrc
│   ├── bash_profile        → ~/.bash_profile  
│   ├── inputrc            → ~/.inputrc
│   └── bash/              # modular components
│       ├── aliases
│       ├── extra
│       ├── init
│       ├── inputrc
│       ├── oh-my-bash
│       ├── prompt
│       ├── rc
│       └── shell
├── zsh/
│   ├── zshrc              → ~/.zshrc
│   ├── inputrc            → ~/.inputrc (alternative)
│   └── zsh/               # modular components
│       ├── aliases
│       ├── extra
│       ├── init
│       ├── inputrc
│       ├── oh-my-zsh
│       ├── prompt
│       ├── rc
│       ├── shell
│       └── zplug
└── fish/
    ├── config.fish        → ~/.config/fish/config.fish
    ├── aliases            → ~/.config/fish/conf.d/aliases.fish
    ├── init              → ~/.config/fish/conf.d/init.fish
    ├── prompt            → ~/.config/fish/functions/fish_prompt.fish
    ├── shell             → ~/.config/fish/conf.d/shell.fish
    └── fish/              # modular components
        ├── extra
        ├── inputrc
        └── oh-my-fish
```

## Naming Convention

### Primary Config Files

| Shell | Asset File      | Target Location                      | Purpose |
|-------|----------------|--------------------------------------|---------|
| Bash  | `bashrc`       | `~/.bashrc`                         | Main bash configuration |
| Bash  | `bash_profile` | `~/.bash_profile`                   | Login shell configuration |
| Bash  | `inputrc`      | `~/.inputrc`                        | Readline configuration |
| Zsh   | `zshrc`        | `~/.zshrc`                          | Main zsh configuration |
| Zsh   | `inputrc`      | `~/.inputrc`                        | Readline configuration (alternative) |
| Fish  | `config.fish`  | `~/.config/fish/config.fish`        | Main fish configuration |

### Modular Components

Each shell has a subdirectory containing modular components that can be sourced:

- **aliases**: Command aliases and shortcuts
- **extra**: User-specific extra configurations  
- **init**: Initialization scripts and environment setup
- **prompt**: Custom prompt configurations
- **rc**: Core runtime configuration
- **shell**: Shell-specific settings and optimizations

### Framework-Specific Components

- **oh-my-bash**: Oh My Bash framework integration
- **oh-my-zsh**: Oh My Zsh framework integration  
- **oh-my-fish**: Oh My Fish framework integration
- **zplug**: Zplug plugin manager for Zsh

## File Permissions

All shell configuration files should use these permissions:

- Config files: `0644` (readable by user and group, writable by user)
- Script files: `0755` (executable by user, readable by all)
- Private files: `0600` (readable/writable by user only)

## Dotfile Conversion Rules

The shell manager converts asset filenames to proper dotfiles:

1. **Direct mapping**: `bashrc` → `.bashrc`
2. **Directory creation**: Fish configs create `~/.config/fish/` directory
3. **Nested paths**: Fish components may create subdirectories like `conf.d/` and `functions/`

## Asset File Requirements

### Mandatory Files

Each shell must have its primary configuration file:

- ✅ `bash/bashrc` 
- ✅ `zsh/zshrc`
- ✅ `fish/config.fish`

### Optional Files

Additional files that enhance functionality:

- `bash_profile` - Login shell setup
- `inputrc` - Readline customization
- Modular components for organization

### File Content Standards

1. **Headers**: Each file should start with a comment header
2. **Sourcing**: Use relative paths for sourcing modular components
3. **Environment**: Set essential environment variables
4. **Compatibility**: Ensure cross-platform compatibility where possible

## Integration with DevEx CLI

### Shell Manager Usage

```bash
# Copy shell configurations
devex system shell copy bash           # Copy bash config
devex system shell copy zsh            # Copy zsh config  
devex system shell copy fish           # Copy fish config
devex system shell copy all            # Copy all available

# Append to configurations
devex system shell append --shell bash --content "export EDITOR=nvim"
devex system shell append --shell zsh --marker "Custom Config" --file ~/.my_config

# Check status
devex system shell status              # Show all shell statuses
devex system shell list               # List available assets
```

### Backup Strategy

Before any modification, the shell manager:

1. Creates automatic backups using the backup system
2. Stores backups with timestamps and descriptions
3. Allows restoration if needed

### Error Handling

The system handles these scenarios gracefully:

- Missing asset files (warns but continues)
- Existing config files (requires `--overwrite` flag)
- Permission issues (provides clear error messages)
- Directory creation failures (creates parent directories)

## Development Guidelines

### Adding New Shell Assets

1. Create the shell directory: `assets/newshell/`
2. Add primary config file: `assets/newshell/newshellrc`
3. Update `shell.go` to include the new shell type
4. Add appropriate tests in `shell_test.go`
5. Update this documentation

### Testing Shell Assets

1. Use temporary directories for testing
2. Verify dotfile name conversion
3. Test backup functionality
4. Ensure proper permissions are set
5. Test append functionality with markers

### Best Practices

1. **Keep it minimal**: Primary configs should source modular components
2. **Use comments**: Document what each section does
3. **Test thoroughly**: Verify on different systems and shells
4. **Backup first**: Always create backups before modifications
5. **Handle errors**: Provide actionable error messages

## Troubleshooting

### Common Issues

1. **Asset not found**: Ensure the asset file exists in the correct location
2. **Permission denied**: Check file/directory permissions
3. **Backup failed**: Verify backup directory is writable
4. **Directory creation failed**: Check parent directory permissions

### Debug Commands

```bash
# Check what's available
devex system shell list

# Check current status
devex system shell status

# Test with dry-run (when implemented)
devex system shell copy bash --dry-run
```

## Future Enhancements

Planned improvements to the shell asset system:

1. **Template system**: Allow parameterized shell configs
2. **Profile support**: Different configs for different use cases
3. **Automatic detection**: Smart detection of installed shells
4. **Migration tools**: Upgrade existing configs
5. **Validation**: Syntax checking before installation

---

This documentation should be kept up-to-date as the shell asset system evolves.
