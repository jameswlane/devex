# Template System

DevEx templates provide pre-configured setups for common development scenarios, enabling rapid environment initialization and team standardization.

## What are Templates?

Templates are YAML configurations that define:
- **Applications** to install for specific tech stacks
- **Programming languages** and their versions
- **Development tools** and utilities
- **System configurations** (git, SSH, terminal)
- **Desktop customizations** (optional)

Templates can be:
- **Built-in** - Maintained by DevEx team
- **Custom** - Created by users or teams
- **Remote** - Shared via URLs or repositories

## Built-in Templates

DevEx includes templates for popular development stacks:

### Web Development
- `web-fullstack` - Full-stack web development (React, Node.js, databases)
- `react-frontend` - React frontend with modern tooling
- `vue-frontend` - Vue.js frontend development
- `nextjs-app` - Next.js application stack
- `backend-api` - Backend API development (Node.js, Python, Go)

### Mobile Development
- `react-native` - React Native development environment
- `flutter` - Flutter development with Android/iOS tools
- `ios-native` - iOS native development (macOS only)
- `android-native` - Android native development

### Data & Analytics
- `data-science` - Python data science stack (pandas, jupyter, etc.)
- `machine-learning` - ML/AI development environment
- `data-engineering` - Big data and pipeline tools

### DevOps & Infrastructure
- `devops-tools` - Infrastructure management and automation
- `kubernetes-dev` - Kubernetes development environment
- `terraform-stack` - Infrastructure as Code with Terraform
- `monitoring-stack` - Observability and monitoring tools

### Language-Specific
- `python-dev` - Python development environment
- `go-dev` - Go development stack
- `rust-dev` - Rust development environment
- `java-enterprise` - Java enterprise development

## Using Templates

### List Available Templates
```bash
# Show all templates
devex template list

# Show templates by category
devex template list --category web

# Include remote templates
devex template list --remote
```

### Apply Template During Init
```bash
# Initialize with template
devex init --template react-fullstack

# Non-interactive template application
devex init --template python-dev --non-interactive
```

### Apply Template to Existing Config
```bash
# Apply template (replaces current config)
devex template apply web-fullstack

# Merge template with current config
devex template apply backend-api --merge

# Apply with backup
devex template apply react-native --backup
```

### Template Information
```bash
# Show template details
devex template info react-fullstack

# Show template content
devex template show react-fullstack
```

## Creating Custom Templates

### From Current Configuration
```bash
# Create template from current setup
devex template create my-stack

# Create with metadata
devex template create my-stack \
  --description "My custom development stack" \
  --category "custom" \
  --tags "web,api,docker"
```

### Manual Template Creation
Create a template file in `~/.devex/templates/`:

```yaml
# ~/.devex/templates/my-stack.yaml
metadata:
  name: "My Development Stack"
  description: "Custom full-stack development environment"
  version: "1.0.0"
  category: "custom"
  tags: ["web", "api", "docker"]
  author: "Your Name"

applications:
  categories:
    development:
      - name: code
        description: Visual Studio Code
        installer: snap
      
      - name: docker
        description: Container platform
        installer: apt
        
    tools:
      - name: postman
        description: API testing
        installer: flatpak

environment:
  languages:
    node:
      version: "18"
      installer: mise
      global_packages:
        - typescript
        - @angular/cli
        - create-react-app
    
    python:
      version: "3.11"
      installer: mise
      packages:
        - fastapi
        - uvicorn

system:
  git:
    user:
      name: "${GIT_USER_NAME}"
      email: "${GIT_USER_EMAIL}"
    core:
      editor: "code --wait"
```

### Template Variables

Templates support variable substitution:

```yaml
# In template
system:
  git:
    user:
      name: "${GIT_USER_NAME}"
      email: "${GIT_USER_EMAIL}"

# Variable file (vars.yaml)
GIT_USER_NAME: "John Doe"
GIT_USER_EMAIL: "john@example.com"

# Apply with variables
devex template apply my-stack --variables vars.yaml
```

### Template Inheritance

Templates can extend other templates:

```yaml
# child-template.yaml
extends: "web-fullstack"

# Additional applications
applications:
  categories:
    development:
      - name: figma
        description: Design tool
        installer: flatpak

# Override environment settings
environment:
  languages:
    node:
      version: "20"  # Override parent version
```

## Team Templates

### Creating Team Templates
```bash
# Create team template with shared config
devex template create company-stack \
  --description "Company standard development stack" \
  --public \
  --category "team"

# Export for distribution
devex config export --template --output company-stack.yaml
```

### Sharing Templates

#### Via File System
```bash
# Export template bundle
devex template export company-stack --output company-template.zip

# Import template bundle
devex template import company-template.zip
```

#### Via URLs
```bash
# Import from URL
devex template import https://company.com/templates/devex-stack.yaml

# Add remote template repository
devex template repo add company https://company.com/devex-templates/
```

#### Version Control Integration
```bash
# Templates can be stored in git repositories
git clone https://github.com/company/devex-templates ~/.devex/remote-templates/company

# Update remote templates
devex template update --remote
```

## Template Management

### Update Templates
```bash
# Update all built-in templates
devex template update

# Update specific template
devex template update web-fullstack

# Update remote templates
devex template update --remote
```

### Template Validation
```bash
# Validate template syntax
devex template validate my-stack

# Test template application
devex template apply my-stack --dry-run

# Validate all templates
devex template validate --all
```

### Remove Templates
```bash
# Remove custom template
devex template remove my-old-stack

# Remove remote template
devex template remove company/legacy-stack
```

## Advanced Template Features

### Conditional Logic
```yaml
# Platform-specific configurations
applications:
  categories:
    development:
      - name: code
        condition: "${OS} != 'windows'"
        installer: snap
      
      - name: code
        condition: "${OS} == 'windows'"
        installer: winget
```

### Template Hooks
```yaml
# Pre/post application hooks
hooks:
  pre_apply:
    - command: "echo 'Setting up development environment...'"
  
  post_apply:
    - command: "code --install-extension ms-python.python"
    - command: "git config --global init.defaultBranch main"
```

### Template Dependencies
```yaml
# Template dependencies
dependencies:
  - name: "base-dev"
    version: ">=1.0.0"
  
  - name: "docker-tools"
    optional: true
```

## Template Examples

### React Full-Stack Template
```yaml
metadata:
  name: "React Full-Stack"
  description: "Complete React development environment with backend tools"
  category: "web"
  tags: ["react", "node", "fullstack"]

applications:
  categories:
    development:
      - name: code
        description: Visual Studio Code
        installer: snap
        extensions:
          - ms-vscode.vscode-typescript-next
          - bradlc.vscode-tailwindcss
      
      - name: docker
        description: Container platform
        installer: apt

environment:
  languages:
    node:
      version: "18"
      installer: mise
      global_packages:
        - create-react-app
        - typescript
        - tailwindcss
    
    python:
      version: "3.11"
      installer: mise
      packages:
        - fastapi
        - uvicorn

system:
  git:
    core:
      editor: "code --wait"
    init:
      defaultBranch: "main"
```

### Data Science Template
```yaml
metadata:
  name: "Data Science"
  description: "Python data science and machine learning environment"
  category: "data"
  tags: ["python", "datascience", "ml"]

applications:
  categories:
    development:
      - name: code
        installer: snap
        extensions:
          - ms-python.python
          - ms-toolsai.jupyter
      
    tools:
      - name: dbeaver
        description: Database tool
        installer: flatpak

environment:
  languages:
    python:
      version: "3.11"
      installer: mise
      packages:
        - jupyter
        - pandas
        - numpy
        - matplotlib
        - scikit-learn
        - tensorflow
    
    r:
      version: "4.3"
      installer: mise
      packages:
        - tidyverse
        - ggplot2
```

## Best Practices

### Template Organization
- Use clear, descriptive names
- Include comprehensive metadata
- Group related applications logically
- Version your templates

### Team Adoption
- Start with built-in templates as base
- Customize gradually based on team needs
- Document template usage and customizations
- Regular template reviews and updates

### Template Maintenance
- Keep templates updated with latest versions
- Test templates regularly
- Use semantic versioning
- Maintain backward compatibility when possible

## Troubleshooting

### Template Not Found
```bash
# Update template list
devex template update

# Check template name spelling
devex template list | grep "template-name"
```

### Application Conflicts
```bash
# Review template content
devex template show template-name

# Apply with merge to resolve conflicts
devex template apply template-name --merge
```

### Variable Substitution Issues
```bash
# Validate variable file
devex template apply template-name --variables vars.yaml --dry-run

# Use default values in template
name: "${GIT_USER_NAME:-Default User}"
```

## Related Commands

- `devex init` - Initialize with templates
- `devex config` - Manage configuration
- `devex install` - Install template applications

## See Also

- [Configuration Guide](config) - Understanding configuration files
- [Team Collaboration](team) - Sharing configurations
- [Command Reference](commands) - All DevEx commands
