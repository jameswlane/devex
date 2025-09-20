# Error Message Standards for DevEx CLI Plugins

This document defines standardized error message formatting for all DevEx CLI plugins to ensure consistency and better user experience.

## General Principles

1. **Follow Go conventions**: lowercase start, no ending punctuation
2. **Be specific and actionable**: include context and suggest solutions when possible
3. **Use consistent terminology**: standardized action words and patterns
4. **Include context**: wrap values in single quotes for clarity
5. **Wrap underlying errors**: use `%w` verb for error chaining

## Standard Patterns

### Input Validation Errors

```go
// Empty/missing values
return fmt.Errorf("URL cannot be empty")
return fmt.Errorf("no applications specified")
return fmt.Errorf("command argument cannot be empty")

// Invalid format/content
return fmt.Errorf("invalid URL format: %w", err)
return fmt.Errorf("invalid application name '%s': must start with alphanumeric character", name)
return fmt.Errorf("URL contains invalid characters")

// Size limits
return fmt.Errorf("URL too long (max %d characters)", maxLen)
return fmt.Errorf("script size %d bytes exceeds maximum allowed %d bytes", actual, max)
```

### Security/Safety Errors

```go
// Dangerous content
return fmt.Errorf("URL contains potentially dangerous character: '%s'", char)
return fmt.Errorf("script contains potentially destructive pattern: %s", description)

// Access restrictions  
return fmt.Errorf("access to localhost is not allowed")
return fmt.Errorf("domain '%s' is not in trusted domains list", domain)
```

### System/External Errors

```go
// Command execution failures
return fmt.Errorf("failed to execute script: %w", err)
return fmt.Errorf("failed to install package '%s': %w", pkg, err)

// Missing dependencies
return fmt.Errorf("curl is not installed or not available in PATH")
return fmt.Errorf("docker daemon is not running")

// Permission issues
return fmt.Errorf("failed to add user to docker group: %w", err)
```

### Plugin/Command Errors

```go
// Unknown commands
return fmt.Errorf("unknown command: '%s'", command)

// Unsupported operations
return fmt.Errorf("unsupported system: no supported package manager found")
```

## Context Guidelines

### Value Formatting
- **Single quotes** around user-provided values: `'%s'`
- **No quotes** around technical details: `%d bytes`, `%w`
- **Descriptive context** for clarity: `domain '%s'`, `package '%s'`

### Action Words
- `failed to X` - for operation failures  
- `cannot X` - for impossible operations
- `invalid X` - for format/content errors
- `unknown X` - for unrecognized values
- `unsupported X` - for unavailable features
- `X is not allowed` - for security restrictions

### Error Wrapping
Always wrap underlying errors with `%w` to preserve error chains:
```go
if err := someOperation(); err != nil {
    return fmt.Errorf("failed to perform operation: %w", err)
}
```

## Examples by Category

### Package Manager Plugins
```go
// Installation
return fmt.Errorf("failed to install package '%s': %w", pkg, err)
return fmt.Errorf("package '%s' not found in repository", pkg)

// Repository operations
return fmt.Errorf("failed to update package lists: %w", err)
return fmt.Errorf("no packages specified for installation")
```

### Tool Plugins
```go
// Tool availability
return fmt.Errorf("git is not installed on this system")
return fmt.Errorf("shell '%s' is not available", shell)

// Configuration
return fmt.Errorf("failed to set configuration '%s': %w", key, err)
```

### Security-focused Plugins
```go
// Content validation
return fmt.Errorf("script contains null bytes")
return fmt.Errorf("script contains excessive command substitution, possible obfuscation")

// URL validation
return fmt.Errorf("URL must use HTTP or HTTPS protocol")
return fmt.Errorf("access to private IP ranges is not allowed")
```

## Implementation Notes

1. **Gradual rollout**: Update error messages as plugins are maintained
2. **Test coverage**: Ensure error message changes are covered by tests
3. **Documentation**: Update user-facing docs when messages change significantly
4. **Backwards compatibility**: Consider impact on any error parsing in client code

This standard ensures all DevEx CLI plugins provide consistent, helpful error messages that follow Go conventions and improve the user experience.
