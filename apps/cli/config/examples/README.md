# DevEx Setup Configuration Examples

This directory contains example setup configurations that demonstrate how to create custom setup workflows for DevEx.

## Available Examples

### 1. `minimal-setup.yaml`
A streamlined setup for developers who want just the essentials:
- Core platform plugins
- One programming language
- Git configuration

**Use case:** Quick setup for new developers or minimal environments

```bash
devex setup --config=config/examples/minimal-setup.yaml
```

### 2. `full-stack-setup.yaml`
Comprehensive setup for full-stack web developers:
- Frontend framework selection (React, Vue, Svelte, Angular)
- Multiple backend languages
- Database systems
- DevOps tools (Docker, Kubernetes, Terraform)
- Shell and theme customization
- Git configuration

**Use case:** Professional full-stack developers setting up a complete environment

```bash
devex setup --config=config/examples/full-stack-setup.yaml
```

## Creating Your Own Setup Configuration

### Configuration Structure

A setup configuration file consists of:

1. **Metadata** - Information about the configuration
2. **Timeouts** - Operation timeouts
3. **Steps** - The workflow screens/questions
4. **Actions** (optional) - Reusable action definitions

### Basic Example

```yaml
metadata:
  name: "My Custom Setup"
  description: "Custom development environment"
  version: "1.0.0"

timeouts:
  plugin_install: 300s
  plugin_verify: 30s
  plugin_download: 120s
  network_operation: 30s

steps:
  - id: welcome
    title: "Welcome"
    type: info
    info:
      message: "Welcome to my custom setup!"
      style: info
    navigation:
      allow_back: false

  - id: language_select
    title: "Programming Language"
    type: question
    question:
      type: select
      variable: language
      prompt: "Choose your language:"
      options:
        - label: "Python"
          value: "python"
        - label: "JavaScript"
          value: "javascript"
      validation:
        required: true
```

### Step Types

#### 1. Info Steps
Display information to the user:

```yaml
- id: welcome
  title: "Welcome"
  type: info
  info:
    message: "Welcome message here"
    style: info  # info, warning, error, success
```

#### 2. Question Steps
Ask the user for input:

```yaml
# Text Input
- id: name
  type: question
  question:
    type: text
    variable: user_name
    prompt: "Enter your name:"
    validation:
      required: true
      min: 2

# Single Select
- id: choice
  type: question
  question:
    type: select
    variable: selection
    prompt: "Choose one:"
    options:
      - label: "Option A"
        value: "a"
      - label: "Option B"
        value: "b"

# Multi-Select
- id: tools
  type: question
  question:
    type: multiselect
    variable: selected_tools
    prompt: "Select tools (Space to select):"
    options:
      - label: "Docker"
        value: "docker"
      - label: "Git"
        value: "git"
    validation:
      min: 1
      max: 5
```

#### 3. Action Steps
Execute installation or configuration:

```yaml
- id: install
  type: action
  action:
    type: install
    progress_message: "Installing..."
    success_message: "Done!"
    on_error: continue  # stop, continue, retry, skip
    params:
      tools: "{{.selected_tools}}"
```

### Conditional Steps

Show steps based on conditions:

```yaml
# Platform-based
- id: linux_only
  type: question
  show_if:
    system:
      os: linux

# Desktop environment
- id: desktop_tools
  type: question
  show_if:
    system:
      has_desktop: true

# Based on previous answers
- id: advanced
  type: question
  show_if:
    variable: experience_level
    operator: equals
    value: advanced

# Complex conditions
- id: conditional
  show_if:
    or:
      - system:
          os: linux
      - and:
          - variable: os_override
            operator: equals
            value: linux
```

### Dynamic Options

Load options from configuration or system:

```yaml
- id: languages
  type: question
  question:
    type: multiselect
    variable: languages
    prompt: "Select languages:"
    options_source:
      type: config
      path: "config/environments"
      key: "languages"
      transform: "get_language_names"

- id: shell
  type: question
  question:
    type: select
    variable: shell
    prompt: "Select shell:"
    options_source:
      type: system
      system_type: shells
```

### Variable Interpolation

Use Go templates to interpolate variables:

```yaml
- id: confirm
  type: info
  info:
    message: |
      You selected:
      • Language: {{.language}}
      • Tools: {{join .selected_tools ", "}}

      {{if .has_errors}}
      Warning: Some errors occurred
      {{end}}
```

### Validation Rules

Add validation to questions:

```yaml
question:
  type: text
  variable: email
  validation:
    required: true
    pattern: '^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$'
    message: "Please enter a valid email"

question:
  type: multiselect
  validation:
    min: 1
    max: 5
    message: "Select between 1 and 5 options"
```

### Navigation Control

Control step navigation:

```yaml
navigation:
  allow_back: true        # Allow going back
  auto_advance: true      # Auto-advance after completion
  next_step: custom_step  # Jump to specific step
  next_step_if:           # Conditional navigation
    language=python: python_setup
    language=go: go_setup
```

## Testing Your Configuration

1. Validate syntax:
```bash
devex setup --config=your-config.yaml --validate
```

2. Dry run (no installations):
```bash
devex setup --config=your-config.yaml --dry-run
```

3. Run with verbose logging:
```bash
devex setup --config=your-config.yaml --verbose
```

## Best Practices

1. **Start Simple** - Begin with a basic configuration and add complexity gradually
2. **Test Thoroughly** - Test your configuration on a clean system or VM
3. **Document Steps** - Add clear descriptions to help users understand each step
4. **Handle Errors** - Use appropriate error handling (`on_error` settings)
5. **Use Conditions** - Skip irrelevant steps based on platform or user selections
6. **Validate Input** - Always validate user input to prevent errors
7. **Provide Feedback** - Use info steps to keep users informed of progress

## Schema Reference

For the complete schema definition, see:
- `internal/types/setup_config.go` - Type definitions
- `internal/config/setup_loader.go` - Loader and validator
- `config/setup.yaml` - Default configuration

## Contributing

To contribute example configurations:

1. Create your configuration in this directory
2. Test it thoroughly
3. Add documentation to this README
4. Submit a pull request

Example configurations should be:
- **Working** - Tested on real systems
- **Documented** - Clear description of purpose and features
- **Useful** - Solve real-world use cases
- **Maintainable** - Use clear, readable YAML
