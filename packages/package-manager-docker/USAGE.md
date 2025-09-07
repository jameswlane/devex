# Docker Package Manager Plugin - Usage Guide

The Docker plugin provides comprehensive container and image management capabilities through DevEx, treating Docker as both a containerization platform and a package manager for containerized applications.

## Table of Contents

- [Installation](#installation)
- [Basic Usage](#basic-usage)
- [Container Management](#container-management)
- [Image Management](#image-management)
- [Docker Compose Integration](#docker-compose-integration)
- [Development Workflows](#development-workflows)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)
- [Examples](#examples)

## Installation

### Prerequisites

- Docker Engine installed on the system
- User added to the `docker` group (for non-root usage)
- Docker daemon running

### Verification

```bash
# Check Docker installation
docker --version
docker info

# Check if Docker plugin is available
devex plugin list | grep package-manager-docker

# Test Docker plugin functionality
devex plugin exec package-manager-docker status
```

### Docker Installation via DevEx

If Docker is not installed, you can install it using DevEx with the APT plugin:

```bash
# Install Docker on Ubuntu/Debian
devex plugin exec package-manager-apt add-repository \
  "https://download.docker.com/linux/ubuntu/gpg" \
  "/usr/share/keyrings/docker-archive-keyring.gpg" \
  "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
  "/etc/apt/sources.list.d/docker.list"

devex plugin exec package-manager-apt update
devex plugin exec package-manager-apt install docker-ce docker-ce-cli containerd.io

# Add user to docker group
sudo usermod -aG docker $USER
newgrp docker
```

## Basic Usage

### Docker Status and Information

```bash
# Check Docker daemon status
devex plugin exec package-manager-docker status

# Ensure Docker is installed and running
devex plugin exec package-manager-docker ensure-installed
```

## Container Management

### Starting and Stopping Containers

```bash
# Start a container
devex plugin exec package-manager-docker start postgres-dev

# Stop a container
devex plugin exec package-manager-docker stop postgres-dev

# Restart a container
devex plugin exec package-manager-docker restart postgres-dev
```

### Listing Containers

```bash
# List running containers
devex plugin exec package-manager-docker list

# List all containers (running and stopped)
devex plugin exec package-manager-docker list --all

# List containers with specific filters
devex plugin exec package-manager-docker list --filter "status=running"
```

### Container Logs and Execution

```bash
# View container logs
devex plugin exec package-manager-docker logs postgres-dev

# Follow logs in real-time
devex plugin exec package-manager-docker logs --follow postgres-dev

# Execute commands in running container
devex plugin exec package-manager-docker exec postgres-dev /bin/bash
devex plugin exec package-manager-docker exec postgres-dev psql -U postgres
```

### Container Lifecycle Management

```bash
# Install (create and start) a new container
devex plugin exec package-manager-docker install postgres:13 \
  --name postgres-dev \
  --env POSTGRES_PASSWORD=devpass \
  --port 5432:5432

# Remove (stop and delete) a container
devex plugin exec package-manager-docker remove postgres-dev
```

## Image Management

### Pulling and Managing Images

```bash
# Pull images from Docker Hub
devex plugin exec package-manager-docker pull postgres:13
devex plugin exec package-manager-docker pull redis:alpine
devex plugin exec package-manager-docker pull node:16-alpine

# Pull from specific registry
devex plugin exec package-manager-docker pull ghcr.io/owner/image:tag
```

### Listing and Removing Images

```bash
# List local images
devex plugin exec package-manager-docker images

# List images with filters
devex plugin exec package-manager-docker images --filter "reference=postgres:*"

# Remove images
devex plugin exec package-manager-docker rmi old-image:tag
devex plugin exec package-manager-docker rmi $(docker images -q --filter "dangling=true")
```

### Building Images

```bash
# Build image from Dockerfile
devex plugin exec package-manager-docker build -t my-app:latest .

# Build with specific Dockerfile
devex plugin exec package-manager-docker build -t my-app:latest -f Dockerfile.prod .

# Build with build arguments
devex plugin exec package-manager-docker build \
  -t my-app:latest \
  --build-arg NODE_ENV=production \
  --build-arg VERSION=1.0.0 \
  .
```

### Pushing Images

```bash
# Push to Docker Hub
devex plugin exec package-manager-docker push my-app:latest

# Push to private registry
devex plugin exec package-manager-docker push registry.company.com/my-app:latest
```

## Docker Compose Integration

### Managing Multi-Container Applications

```bash
# Start services defined in docker-compose.yml
devex plugin exec package-manager-docker compose up

# Start services in background (detached mode)
devex plugin exec package-manager-docker compose up -d

# Start specific services
devex plugin exec package-manager-docker compose up web database

# Stop and remove services
devex plugin exec package-manager-docker compose down

# Stop and remove services with volumes
devex plugin exec package-manager-docker compose down -v
```

### Service Management

```bash
# View service logs
devex plugin exec package-manager-docker compose logs
devex plugin exec package-manager-docker compose logs web

# Scale services
devex plugin exec package-manager-docker compose scale web=3
devex plugin exec package-manager-docker compose scale worker=2

# Execute commands in services
devex plugin exec package-manager-docker compose exec web /bin/bash
devex plugin exec package-manager-docker compose exec database psql -U postgres
```

### Development Workflows

```bash
# Start development environment
devex plugin exec package-manager-docker compose -f docker-compose.dev.yml up -d

# Rebuild services after code changes
devex plugin exec package-manager-docker compose up --build

# View service status
devex plugin exec package-manager-docker compose ps
```

## Development Workflows

### Database Development Environment

#### PostgreSQL Development Setup
```bash
# Create PostgreSQL development container
devex plugin exec package-manager-docker install postgres:13 \
  --name postgres-dev \
  --env POSTGRES_DB=devdb \
  --env POSTGRES_USER=devuser \
  --env POSTGRES_PASSWORD=devpass \
  --port 5432:5432 \
  --volume postgres-data:/var/lib/postgresql/data

# Verify PostgreSQL is running
devex plugin exec package-manager-docker logs postgres-dev

# Connect to PostgreSQL
devex plugin exec package-manager-docker exec postgres-dev psql -U devuser -d devdb
```

#### Redis Development Setup
```bash
# Create Redis development container
devex plugin exec package-manager-docker install redis:alpine \
  --name redis-dev \
  --port 6379:6379 \
  --volume redis-data:/data

# Test Redis connection
devex plugin exec package-manager-docker exec redis-dev redis-cli ping
```

#### MySQL Development Setup
```bash
# Create MySQL development container
devex plugin exec package-manager-docker install mysql:8.0 \
  --name mysql-dev \
  --env MYSQL_ROOT_PASSWORD=rootpass \
  --env MYSQL_DATABASE=devdb \
  --env MYSQL_USER=devuser \
  --env MYSQL_PASSWORD=devpass \
  --port 3306:3306 \
  --volume mysql-data:/var/lib/mysql

# Connect to MySQL
devex plugin exec package-manager-docker exec mysql-dev mysql -u devuser -p devdb
```

### Full-Stack Development Environment

#### LAMP Stack with Docker Compose
```yaml
# docker-compose.yml
version: '3.8'
services:
  web:
    build: .
    ports:
      - "80:80"
    volumes:
      - ./src:/var/www/html
    depends_on:
      - database
  
  database:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: rootpass
      MYSQL_DATABASE: appdb
      MYSQL_USER: appuser
      MYSQL_PASSWORD: apppass
    volumes:
      - mysql-data:/var/lib/mysql
    ports:
      - "3306:3306"

volumes:
  mysql-data:
```

```bash
# Start LAMP development environment
devex plugin exec package-manager-docker compose up -d

# View logs
devex plugin exec package-manager-docker compose logs -f web

# Access web container
devex plugin exec package-manager-docker compose exec web /bin/bash
```

#### MEAN Stack Setup
```yaml
# docker-compose.yml for MEAN stack
version: '3.8'
services:
  mongo:
    image: mongo:5.0
    environment:
      MONGO_INITDB_ROOT_USERNAME: admin
      MONGO_INITDB_ROOT_PASSWORD: adminpass
      MONGO_INITDB_DATABASE: appdb
    ports:
      - "27017:27017"
    volumes:
      - mongo-data:/data/db
  
  node-app:
    build: 
      context: .
      dockerfile: Dockerfile.dev
    ports:
      - "3000:3000"
    volumes:
      - .:/app
      - /app/node_modules
    depends_on:
      - mongo
    environment:
      - NODE_ENV=development
      - MONGODB_URI=mongodb://admin:adminpass@mongo:27017/appdb

volumes:
  mongo-data:
```

```bash
# Start MEAN development environment
devex plugin exec package-manager-docker compose up -d

# Install npm dependencies in container
devex plugin exec package-manager-docker compose exec node-app npm install

# Run development server
devex plugin exec package-manager-docker compose exec node-app npm run dev
```

### Microservices Development

```bash
# Create network for microservices
docker network create microservices-net

# Start API gateway
devex plugin exec package-manager-docker install nginx:alpine \
  --name api-gateway \
  --port 80:80 \
  --network microservices-net

# Start user service
devex plugin exec package-manager-docker install node:16-alpine \
  --name user-service \
  --network microservices-net \
  --volume ./user-service:/app \
  --workdir /app \
  --command "npm run dev"

# Start order service
devex plugin exec package-manager-docker install node:16-alpine \
  --name order-service \
  --network microservices-net \
  --volume ./order-service:/app \
  --workdir /app \
  --command "npm run dev"
```

## Configuration

### Plugin Configuration

```yaml
# ~/.devex/config.yaml
package_managers:
  docker:
    daemon_socket: "/var/run/docker.sock"
    default_registry: "docker.io"
    compose_version: "v2"
    build_context: "."
    default_network: "bridge"
    auto_pull_latest: false
    cleanup_dangling: true
    timeout: 300
```

### Environment Variables

```bash
# Docker plugin configuration
export DEVEX_DOCKER_HOST=unix:///var/run/docker.sock
export DEVEX_DOCKER_REGISTRY=docker.io
export DEVEX_DOCKER_TIMEOUT=300
export DEVEX_DOCKER_COMPOSE_VERSION=v2
export DEVEX_DOCKER_AUTO_PULL=false
```

### Docker Configuration

```bash
# Configure Docker daemon (in /etc/docker/daemon.json)
{
  "storage-driver": "overlay2",
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  },
  "default-address-pools": [
    {
      "base": "172.20.0.0/16",
      "size": 24
    }
  ]
}
```

## Troubleshooting

### Common Issues

#### Docker Daemon Not Running
```bash
# Error: Cannot connect to the Docker daemon
# Solution: Start Docker daemon
sudo systemctl start docker
sudo systemctl enable docker

# Check daemon status
devex plugin exec package-manager-docker status
```

#### Permission Denied
```bash
# Error: permission denied while trying to connect to Docker daemon
# Solution: Add user to docker group
sudo usermod -aG docker $USER
newgrp docker

# Verify group membership
groups | grep docker
```

#### Container Won't Start
```bash
# Debug container startup issues
devex plugin exec package-manager-docker logs container-name

# Check container configuration
docker inspect container-name

# Start with interactive mode for debugging
docker run -it --rm image-name /bin/bash
```

#### Port Conflicts
```bash
# Error: port already in use
# Solution: Find and stop conflicting process
sudo lsof -i :5432
sudo kill -9 PID

# Or use different port
devex plugin exec package-manager-docker install postgres:13 \
  --port 5433:5432
```

#### Out of Disk Space
```bash
# Clean up unused resources
docker system prune -a

# Remove unused volumes
docker volume prune

# Remove unused networks
docker network prune
```

### Debug Information

```bash
# Show Docker system information
docker system info
docker system df

# Show plugin status
devex plugin status package-manager-docker

# Generate debug report
devex debug docker --output docker-debug.log
```

### Performance Optimization

```bash
# Optimize Docker performance
# 1. Use multi-stage builds
# 2. Minimize layers in Dockerfile
# 3. Use .dockerignore file
# 4. Clean up regularly

# Enable Docker BuildKit for faster builds
export DOCKER_BUILDKIT=1
```

## Examples

### Development Database Stack

```bash
#!/bin/bash
# setup-dev-databases.sh - Set up development databases

# PostgreSQL
devex plugin exec package-manager-docker install postgres:13 \
  --name postgres-dev \
  --env POSTGRES_DB=devdb \
  --env POSTGRES_USER=devuser \
  --env POSTGRES_PASSWORD=devpass \
  --port 5432:5432 \
  --volume postgres-data:/var/lib/postgresql/data

# Redis
devex plugin exec package-manager-docker install redis:alpine \
  --name redis-dev \
  --port 6379:6379 \
  --volume redis-data:/data

# MongoDB
devex plugin exec package-manager-docker install mongo:5.0 \
  --name mongo-dev \
  --env MONGO_INITDB_ROOT_USERNAME=admin \
  --env MONGO_INITDB_ROOT_PASSWORD=adminpass \
  --port 27017:27017 \
  --volume mongo-data:/data/db

echo "Development databases started successfully!"
echo "PostgreSQL: localhost:5432 (devuser/devpass)"
echo "Redis: localhost:6379"
echo "MongoDB: localhost:27017 (admin/adminpass)"
```

### Container Health Monitoring

```bash
#!/bin/bash
# monitor-containers.sh - Monitor container health

containers=("postgres-dev" "redis-dev" "mongo-dev")

for container in "${containers[@]}"; do
    if devex plugin exec package-manager-docker list | grep -q $container; then
        echo "✅ $container is running"
        
        # Show resource usage
        docker stats $container --no-stream
    else
        echo "❌ $container is not running"
    fi
done
```

### Development Environment Cleanup

```bash
#!/bin/bash
# cleanup-dev-env.sh - Clean up development environment

# Stop and remove development containers
containers=("postgres-dev" "redis-dev" "mongo-dev" "nginx-dev")

for container in "${containers[@]}"; do
    echo "Stopping $container..."
    devex plugin exec package-manager-docker stop $container 2>/dev/null
    devex plugin exec package-manager-docker remove $container 2>/dev/null
done

# Clean up unused resources
echo "Cleaning up unused Docker resources..."
docker system prune -f
docker volume prune -f
docker network prune -f

echo "Development environment cleanup completed!"
```

## Best Practices

1. **Use specific image tags**: Avoid `latest` tag in production
2. **Implement health checks**: Add health checks to containers
3. **Use multi-stage builds**: Optimize image sizes
4. **Volume management**: Use named volumes for persistent data
5. **Network isolation**: Use custom networks for service communication
6. **Resource limits**: Set memory and CPU limits for containers
7. **Security scanning**: Regularly scan images for vulnerabilities
8. **Log management**: Configure proper logging drivers
9. **Backup strategies**: Implement backup procedures for persistent data
10. **Documentation**: Document container configurations and dependencies

## Integration with DevEx CLI

The Docker plugin integrates seamlessly with DevEx's main CLI:

```bash
# Use through DevEx main CLI (if Docker is detected as available package manager)
devex install postgres:13  # May use Docker plugin for containerized installation

# Direct plugin usage for container management
devex plugin exec package-manager-docker install postgres:13

# Combined workflow
devex system detect  # Detects Docker as available
devex install --package-manager docker postgres:13  # Force Docker usage
```

For more information about DevEx and other plugins, see the main [USAGE.md](../../USAGE.md) documentation.
