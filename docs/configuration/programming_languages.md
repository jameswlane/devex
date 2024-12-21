
---

### File: `docs/configuration/programming_languages.md`

```markdown
# Programming Languages Configuration

This file (`config/programming_languages.yaml`) defines the programming languages that can be installed using the CLI.

---

## Schema
- **`name`**: The name of the programming language.
- **`description`**: A short description.
- **`install_method`**: How the language is installed (`mise`, `apt`, etc.).
- **`dependencies`**: Libraries or tools required for setup.
- **`post_install`**: Commands to run after installation.

### Example:
```yaml
- name: Go
  description: "Go programming language"
  category: "Programming Languages"
  install_method: "mise"
  install_command: "go@latest"
  uninstall_command: "go"
  default: true
  dependencies:
    - "mise"
  post_install:
    - command: "code --install-extension golang.go"
