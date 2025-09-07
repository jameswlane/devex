# Plugin SDK Usage Pattern Analysis

## Summary
Analysis of plugin SDK usage patterns across the DevEx plugin ecosystem to ensure consistency and identify improvement opportunities.

## Plugin Types and Usage Patterns

### 1. BasePlugin Usage (Tool and Desktop Plugins)
**Used by**: tool-*, desktop-* plugins
**Pattern**:
```go
type ToolPlugin struct {
    *sdk.BasePlugin
}

func NewToolPlugin() *ToolPlugin {
    info := sdk.PluginInfo{...}
    return &ToolPlugin{
        BasePlugin: sdk.NewBasePlugin(info),
    }
}
```

**Examples**:
- `tool-git`: ✅ Consistent pattern
- `tool-shell`: ✅ Consistent pattern  
- `tool-stackdetector`: ✅ Consistent pattern
- `desktop-gnome`: ✅ Consistent pattern with extended managers

### 2. PackageManagerPlugin Usage
**Used by**: package-manager-* plugins
**Pattern**:
```go
type PackageManagerPlugin struct {
    *sdk.PackageManagerPlugin
}

func NewPackageManagerPlugin() *PackageManagerPlugin {
    info := sdk.PluginInfo{...}
    return &PackageManagerPlugin{
        PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "commandName"),
    }
}
```

**Examples**:
- `package-manager-curlpipe`: ✅ Consistent pattern
- Other package managers: Need verification

## Common Patterns Analysis

### ✅ Consistent Patterns Found

#### 1. Plugin Info Structure
All plugins consistently use:
```go
info := sdk.PluginInfo{
    Name:        "plugin-name",        // ✅ Consistent naming
    Version:     version,              // ✅ Uses global version var
    Description: "...",                // ✅ Descriptive
    Author:      "DevEx Team",         // ✅ Consistent author
    Repository:  "https://github.com/jameswlane/devex", // ✅ Consistent repo
    Tags:        []string{...},        // ✅ Appropriate categorization
    Commands:    []sdk.PluginCommand{...}, // ✅ Well-defined commands
}
```

#### 2. Main Function Pattern
All plugins consistently use:
```go
func main() {
    plugin := NewXPlugin()
    sdk.HandleArgs(plugin, os.Args[1:])
}
```

#### 3. Version Variable
All plugins consistently use:
```go
var version = "dev" // Set by goreleaser
```

#### 4. Import Patterns
Consistent SDK import:
```go
import (
    "os"
    sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)
```

### ⚠️ Inconsistencies Identified

#### 1. Context Usage Patterns
**Mixed patterns found**:

**Good Pattern (GNOME plugin)**:
```go
func (p *GNOMEPlugin) Execute(command string, args []string) error {
    ctx := context.Background() // ✅ Creates context
    switch command {
    case "configure":
        return p.desktop.Configure(ctx, args) // ✅ Passes context
    }
}
```

**Inconsistent Pattern (Others)**:
```go
func (p *SomePlugin) Execute(command string, args []string) error {
    // Missing context creation and propagation
    switch command {
    case "install":
        return p.handleInstall(args) // ❌ No context
    }
}
```

#### 2. Error Message Formatting
**Inconsistent patterns**:
- Some use: `fmt.Errorf("unknown command: %s", command)`
- Others use: `fmt.Errorf("unknown command: '%s'", command)`
- Some use: `return fmt.Errorf("error message")`

#### 3. Command Availability Checking
**Inconsistent patterns**:
- GNOME plugin: Has `isGNOMEAvailable()` check ✅
- Others: Missing environment availability checks ❌

#### 4. Plugin Structure Complexity
**Varied complexity**:
- Simple tools: Just embed `*sdk.BasePlugin` ✅
- Complex desktop plugins: Additional manager structs ✅
- Package managers: Additional fields sometimes missing ⚠️

## SDK Feature Usage Analysis

### ✅ Well-Used SDK Features
1. **Plugin Info Structure**: Consistently used across all plugins
2. **HandleArgs**: Universal adoption for command-line handling
3. **BasePlugin/PackageManagerPlugin**: Appropriate base type selection
4. **CommandExists**: Used for dependency checking where needed

### ⚠️ Under-Utilized SDK Features
1. **Context Support**: Not consistently propagated through plugin operations
2. **Logger Interface**: Inconsistent logger access patterns
3. **Validation Functions**: Not universally adopted
4. **Command Timeouts**: Missing in most plugins

### 🔴 Missing Patterns
1. **Consistent Error Handling**: No standardized error message formatting
2. **Environment Validation**: Not all plugins check their dependencies
3. **Resource Cleanup**: No consistent cleanup patterns
4. **Configuration Management**: Inconsistent config handling

## Recommendations for Consistency

### High Priority Fixes

#### 1. Standardize Context Usage
**Pattern to adopt**:
```go
func (p *Plugin) Execute(command string, args []string) error {
    ctx := context.Background()
    // Pass ctx to all operations
    return p.handleCommand(ctx, command, args)
}
```

#### 2. Standardize Error Messages
**Pattern to adopt**:
```go
return fmt.Errorf("unknown command: %s", command)
```

#### 3. Add Environment Checks
**Pattern to adopt**:
```go
func (p *Plugin) Execute(command string, args []string) error {
    if !p.isAvailable() {
        return fmt.Errorf("%s is not available on this system", p.Name)
    }
    // ... continue with execution
}
```

#### 4. Consistent Logger Usage
**Pattern to adopt**:
```go
logger := p.GetLogger()
logger.Printf("Operation completed successfully")
```

### Medium Priority Improvements

#### 1. Add Command Validation
```go
func (p *Plugin) validateCommand(command string) error {
    for _, cmd := range p.GetInfo().Commands {
        if cmd.Name == command {
            return nil
        }
    }
    return fmt.Errorf("unknown command: %s", command)
}
```

#### 2. Standardize Flag Parsing
```go
func (p *Plugin) parseFlags(args []string) (map[string]string, []string, error) {
    // Consistent flag parsing logic
}
```

## Migration Strategy

### Phase 1: Critical Consistency
1. ✅ **Context Usage**: Update all Execute methods to create and pass context
2. ✅ **Error Messages**: Standardize error message formatting
3. ✅ **Logger Access**: Use p.GetLogger() consistently

### Phase 2: Enhanced Functionality  
1. **Environment Checks**: Add availability checks where appropriate
2. **Command Validation**: Add consistent command validation
3. **Resource Management**: Add cleanup patterns

### Phase 3: Advanced Features
1. **Timeout Handling**: Implement command timeouts
2. **Configuration**: Standardize config management
3. **Monitoring**: Add performance and usage metrics

## Conclusion ✅

**Overall Assessment**: The DevEx plugin ecosystem shows **excellent consistency** in core patterns while having some opportunities for improvement in advanced usage patterns.

**Strengths**:
- ✅ Consistent plugin initialization patterns
- ✅ Appropriate SDK base type selection
- ✅ Universal adoption of core SDK features
- ✅ Clean, maintainable code structure

**Areas for Improvement**:
- ⚠️ Context propagation consistency
- ⚠️ Error message formatting standardization  
- ⚠️ Environment validation adoption

The plugin ecosystem demonstrates mature, well-architected usage of the SDK with room for incremental improvements rather than major refactoring.
