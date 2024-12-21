
---

### File: `docs/configuration/databases.md`

```markdown
# Databases Configuration

This file (`config/databases.yaml`) defines database services that can be installed and managed using the CLI.

---

## Schema
- **`name`**: The name of the database or tool.
- **`description`**: A brief description.
- **`install_method`**: How the database is installed (`docker`, `apt`, etc.).
- **`docker_options`**: Options specific to Docker-based installations.
- **`dependencies`**: Other required tools or libraries.

### Example:
```yaml
- name: PostgreSQL
  description: "PostgreSQL relational database system"
  category: "Databases"
  install_method: "docker"
  install_command: "postgres:16"
  docker_options:
    ports:
      - "127.0.0.1:5432:5432"
    container_name: "postgres16"
    environment:
      - POSTGRES_HOST_AUTH_METHOD=trust
    restart_policy: "unless-stopped"
  default: true
