# DevEx Plugin Registry

[![Next.js](https://img.shields.io/badge/Next.js-15.5-black?logo=next.js)](https://nextjs.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.x-blue?logo=typescript)](https://www.typescriptlang.org/)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../LICENSE)
[![Registry](https://img.shields.io/badge/Plugin%20Registry-36%20Plugins-green)](https://registry.devex.sh)

Centralized plugin registry and distribution platform for DevEx plugins. Manages the lifecycle, versioning, and distribution of 36+ specialized plugins for package managers and desktop environments.

## 🚀 Features

- **📦 Plugin Discovery**: Browse and search all available DevEx plugins
- **🔄 Version Management**: Track plugin versions, changelogs, and compatibility
- **📊 Analytics Dashboard**: Monitor plugin usage, downloads, and performance
- **🏗️ Automated Updates**: Continuous integration with plugin builds
- **📱 Responsive Design**: Modern, mobile-first interface
- **🔍 Advanced Search**: Filter plugins by category, platform, and features

## 🏗️ Architecture

### Registry Structure
```
apps/registry/
├── src/
│   ├── app/                 # Next.js 15 App Router
│   │   ├── (dashboard)/     # Dashboard route group
│   │   ├── api/            # API routes for plugin data
│   │   ├── plugin/         # Individual plugin pages
│   │   └── layout.tsx      # Root layout
│   ├── components/         # Registry UI components
│   │   ├── PluginCard/     # Plugin display cards
│   │   ├── SearchFilter/   # Search and filtering
│   │   ├── Analytics/      # Usage analytics
│   │   └── VersionHistory/ # Version tracking
│   ├── lib/               # Utilities and API clients
│   │   ├── registry/      # Plugin registry logic
│   │   ├── analytics/     # Analytics processing
│   │   └── api/          # API client functions
│   └── types/            # TypeScript definitions
├── public/              # Static assets and plugin metadata
│   ├── plugins/         # Plugin information and assets
│   └── icons/          # Plugin icons and badges
├── data/               # Plugin registry data
│   ├── plugins.json    # Plugin metadata
│   └── versions.json   # Version history
└── package.json       # Dependencies and scripts
```

### Technology Stack
- **Next.js 15**: React framework with App Router
- **TypeScript**: Type-safe development
- **Tailwind CSS**: Utility-first styling
- **Prisma**: Database ORM for plugin data
- **PostgreSQL**: Production database
- **Vercel**: Hosting and deployment

## 🔌 Plugin Registry System

### Plugin Categories

#### Package Manager Plugins (23)
- **Linux**: apt, dnf, pacman, yay, zypper, emerge, eopkg, xbps, apk, rpm, deb
- **Universal**: flatpak, snap, appimage, docker, pip, mise
- **Cross-platform**: brew
- **Direct**: curlpipe
- **Nix**: nixpkgs, nixflake

#### Desktop Environment Plugins (9)
- **GNOME**, **KDE Plasma**, **XFCE**, **MATE**, **Cinnamon**
- **LXQt**, **Budgie**, **Pantheon**, **COSMIC**

#### System Tool Plugins (4)
- **tool-git**: Git configuration and credential management
- **tool-shell**: Shell setup and configuration
- **tool-stackdetector**: Automatic project stack detection
- **system-setup**: Core system optimization and setup

### Plugin Metadata Schema
```typescript
interface Plugin {
  id: string
  name: string
  description: string
  category: 'package-manager' | 'desktop' | 'system-tool'
  type: string
  version: string
  platforms: Platform[]
  maintainer: string
  repository: string
  documentation: string
  downloads: number
  rating: number
  tags: string[]
  createdAt: Date
  updatedAt: Date
}

interface PluginVersion {
  version: string
  changelog: string
  compatibility: string[]
  releaseDate: Date
  downloadUrl: string
  checksum: string
}
```

## 🚀 Quick Start

### Development Setup
```bash
# Install dependencies
pnpm install

# Set up database
pnpm db:setup

# Start development server
pnpm dev

# Open http://localhost:3000
```

### Environment Configuration
```bash
# Development
DATABASE_URL="postgresql://localhost:5432/devex_registry_dev"
NEXT_PUBLIC_REGISTRY_URL="http://localhost:3000"

# Production
DATABASE_URL="postgresql://prod-db/devex_registry"
NEXT_PUBLIC_REGISTRY_URL="https://registry.devex.sh"
```

## 📦 Plugin Management

### Plugin Registration
```typescript
// Register a new plugin
const plugin = await registerPlugin({
  name: 'package-manager-apt',
  description: 'Advanced Package Tool for Debian/Ubuntu',
  category: 'package-manager',
  type: 'apt',
  platforms: ['linux'],
  repository: 'https://github.com/jameswlane/devex',
  maintainer: 'James Lane'
})
```

### Version Management
```typescript
// Release new plugin version
const version = await releaseVersion({
  pluginId: 'package-manager-apt',
  version: '1.2.0',
  changelog: 'Added support for Ubuntu 24.04',
  compatibility: ['ubuntu-24.04', 'debian-12'],
  binary: './dist/devex-plugin-package-manager-apt'
})
```

### Plugin Discovery
```typescript
// Search plugins
const plugins = await searchPlugins({
  category: 'package-manager',
  platforms: ['linux'],
  query: 'ubuntu'
})

// Get plugin details
const plugin = await getPlugin('package-manager-apt')
const versions = await getPluginVersions('package-manager-apt')
```

## 🎨 User Interface

### Plugin Discovery Page
- **Grid/List View**: Toggle between card grid and detailed list
- **Advanced Filtering**: Filter by category, platform, rating, and tags
- **Search**: Real-time search across plugin names and descriptions
- **Sorting**: Sort by popularity, rating, recent updates, or alphabetical

### Plugin Detail Page
- **Overview**: Description, screenshots, and key features
- **Installation**: Copy-paste installation commands
- **Documentation**: Links to plugin-specific documentation
- **Versions**: Version history with changelogs
- **Analytics**: Download statistics and usage trends
- **Community**: Ratings, reviews, and issue tracking

### Analytics Dashboard
- **Registry Metrics**: Total plugins, downloads, and active users
- **Popular Plugins**: Most downloaded and highest rated
- **Platform Distribution**: Usage across different platforms
- **Growth Trends**: Plugin adoption and usage patterns

## 📊 Analytics and Monitoring

### Plugin Analytics
```typescript
interface PluginAnalytics {
  pluginId: string
  downloads: {
    total: number
    daily: number[]
    weekly: number[]
    monthly: number[]
  }
  platforms: {
    [platform: string]: number
  }
  versions: {
    [version: string]: number
  }
  userRating: {
    average: number
    distribution: number[]
  }
}
```

### Usage Tracking
- **Download Metrics**: Track plugin downloads by version and platform
- **User Analytics**: Anonymous usage statistics and patterns  
- **Performance Monitoring**: Plugin load times and success rates
- **Error Tracking**: Monitor plugin installation failures

## 🔄 Automated Updates

### CI/CD Integration
```yaml
# .github/workflows/registry-update.yml
name: Update Plugin Registry
on:
  workflow_run:
    workflows: ["Plugin Build"]
    types: [completed]

jobs:
  update-registry:
    runs-on: ubuntu-latest
    steps:
      - name: Update Plugin Metadata
        run: |
          # Update plugin versions
          # Generate new metadata
          # Deploy to registry
```

### Plugin Lifecycle
1. **Development**: Plugin development in individual repositories
2. **Build**: Automated builds via GitHub Actions
3. **Registry Update**: Automatic metadata and version updates
4. **Distribution**: Binary distribution through registry
5. **Analytics**: Usage tracking and feedback collection

## 🛡️ Security and Validation

### Plugin Validation
```typescript
interface ValidationRules {
  binarySignature: boolean
  checksumVerification: boolean
  securityScan: boolean
  compatibilityTest: boolean
  performanceTest: boolean
}

// Validate plugin before registry acceptance
const validation = await validatePlugin(plugin, {
  binarySignature: true,
  checksumVerification: true,
  securityScan: true,
  compatibilityTest: true,
  performanceTest: false
})
```

### Security Measures
- **Binary Signing**: All plugin binaries are cryptographically signed
- **Checksum Verification**: SHA-256 checksums for integrity verification
- **Security Scanning**: Automated vulnerability scanning
- **Sandboxed Testing**: Safe testing environment for plugin validation
- **Rate Limiting**: API rate limiting to prevent abuse

## 🔍 API Reference

### Registry API
```typescript
// GET /api/plugins - List all plugins
interface PluginsResponse {
  plugins: Plugin[]
  pagination: {
    page: number
    limit: number
    total: number
  }
}

// GET /api/plugins/:id - Get plugin details
interface PluginResponse {
  plugin: Plugin
  versions: PluginVersion[]
  analytics: PluginAnalytics
}

// POST /api/plugins/:id/download - Track download
interface DownloadResponse {
  downloadUrl: string
  checksum: string
  expires: Date
}
```

### Search API
```typescript
// GET /api/search - Search plugins
interface SearchRequest {
  query?: string
  category?: string[]
  platforms?: string[]
  tags?: string[]
  sort?: 'popularity' | 'rating' | 'updated' | 'name'
  page?: number
  limit?: number
}
```

## 🚀 Deployment

### Vercel Configuration
```json
{
  "buildCommand": "pnpm build",
  "outputDirectory": ".next",
  "framework": "nextjs",
  "functions": {
    "src/app/api/**": {
      "runtime": "nodejs18.x",
      "maxDuration": 30
    }
  },
  "env": {
    "DATABASE_URL": "@database-url",
    "REGISTRY_SECRET": "@registry-secret"
  }
}
```

### Database Migration
```bash
# Run database migrations
pnpm db:migrate

# Seed initial plugin data
pnpm db:seed

# Generate Prisma client
pnpm db:generate
```

## 📱 Mobile Experience

### Progressive Web App
- **Offline Support**: Cache plugin metadata for offline browsing
- **App-like Experience**: Native app feel with PWA capabilities
- **Push Notifications**: Updates about new plugins and versions
- **Quick Actions**: Direct access to frequently used plugins

### Mobile Optimization
- **Touch-Friendly**: Large touch targets and gesture support
- **Fast Loading**: Optimized for mobile networks
- **Responsive Design**: Adapts to all screen sizes
- **Dark Mode**: Built-in dark/light theme switching

## 🤝 Contributing

### Registry Development
1. Fork the repository
2. Set up local development environment
3. Create feature branch: `git checkout -b feat/registry-feature`
4. Make changes following coding standards
5. Test locally: `pnpm test && pnpm build`
6. Submit Pull Request with clear description

### Plugin Submission
1. Develop plugin following DevEx plugin standards
2. Create comprehensive documentation
3. Submit plugin metadata via API or GitHub issue
4. Await validation and security review
5. Plugin appears in registry upon approval

## 📄 License

The DevEx Plugin Registry is licensed under the [Apache-2.0 License](../../LICENSE).

---

<div align="center">

**[Visit Registry](https://registry.devex.sh)** • **[Submit Plugin](https://github.com/jameswlane/devex/issues/new?template=plugin-submission.md)** • **[API Docs](https://registry.devex.sh/docs)**

</div>
