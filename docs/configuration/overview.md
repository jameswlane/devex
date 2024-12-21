# CLI Configuration Overview

The DevEx CLI supports a wide range of configurations for apps, programming languages, databases, and more. These configurations are stored in YAML files and mapped to structured types within the CLI's internal logic.

---

## Configuration Files

1. **`config/apps.yaml`**: Defines the applications that can be installed.
2. **`config/programming_languages.yaml`**: Manages programming language environments.
3. **`config/databases.yaml`**: Configures database-related services.

Each file uses the following fields to define resources:

### Common Fields:
- **`name`**: The name of the app, language, or database.
- **`description`**: A brief description of the item.
- **`install_method`**: Specifies how the item is installed (e.g., `apt`, `docker`, `manual`).
- **`install_command`**: The command used for installation.
- **`dependencies`**: List of other items that must be installed first.

### Example:
```yaml
- name: Node.js
  description: "Node.js runtime"
  category: "Programming Languages"
  install_method: "mise"
  install_command: "node@lts"
  uninstall_command: "node"
  dependencies:
    - "mise"
  default: true
