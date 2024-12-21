
---

### File: `docs/configuration/apps.md`

```markdown
# Applications Configuration

This file (`config/apps.yaml`) defines the list of optional applications that can be installed using the CLI.

---

## Schema
- **`name`**: The name of the application.
- **`description`**: A short description of the application.
- **`category`**: The category (e.g., `Development`, `Utilities`).
- **`install_method`**: How the application is installed (`apt`, `manual`, `docker`, etc.).
- **`dependencies`**: Other required apps or libraries.

### Example:
```yaml
- name: GitHub CLI
  description: "A command-line tool for GitHub"
  category: "Development Tools"
  install_method: "apt"
  install_command: "gh"
  uninstall_command: "gh"
  dependencies:
    - "curl"
    - "apt"
