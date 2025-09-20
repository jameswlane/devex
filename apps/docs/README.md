# DevEx Documentation

[![Next.js](https://img.shields.io/badge/Next.js-15.5-black?logo=next.js)](https://nextjs.org/)
[![Fumadocs](https://img.shields.io/badge/Fumadocs-MDX-blue)](https://fumadocs.dev/)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../LICENSE)
[![Vercel](https://img.shields.io/badge/Deployed%20on-Vercel-black?logo=vercel)](https://docs.devex.sh)

Comprehensive technical documentation for DevEx built with Next.js 15, Fumadocs, and MDX. Hosted at [docs.devex.sh](https://docs.devex.sh).

## 🚀 Features

- **📖 Fumadocs MDX**: Advanced MDX with React components and syntax highlighting
- **🎨 Modern Design**: Clean, responsive documentation interface with dark/light mode
- **🔍 Full-Text Search**: Lightning-fast search across all documentation
- **📱 Mobile-First**: Responsive design optimized for all devices
- **🚀 Static Generation**: Pre-rendered for optimal performance
- **🔄 Auto-Deploy**: Automatic deployments via Vercel

## 🏗️ Architecture

### Documentation Structure
```
apps/docs/
├── app/
│   ├── (home)/              # Landing page route group
│   ├── docs/               # Documentation pages
│   │   └── [[...slug]]/    # Dynamic routing for all docs
│   ├── api/
│   │   └── search/         # Search API endpoint
│   ├── layout.tsx          # Root layout
│   └── layout.config.tsx   # Fumadocs layout configuration
├── content/               # MDX documentation files
│   ├── docs/
│   │   ├── getting-started/
│   │   ├── cli-reference/
│   │   ├── plugin-development/
│   │   └── api/
│   └── meta.json         # Navigation structure
├── lib/
│   └── source.ts         # Content source adapter
├── source.config.ts      # MDX configuration
└── package.json         # Dependencies and scripts
```

### Technology Stack
- **Next.js 15**: React framework with App Router
- **Fumadocs**: Documentation framework with MDX support
- **MDX**: Markdown with embedded React components
- **Tailwind CSS**: Utility-first styling with dark mode
- **TypeScript**: Type-safe development
- **Vercel**: Hosting and deployment platform

## 🚀 Quick Start

### Development Setup
```bash
# Install dependencies
pnpm install

# Start development server
pnpm dev

# Open http://localhost:3000
```

### Build and Deploy
```bash
# Build for production
pnpm build

# Start production server
pnpm start

# Type check
pnpm type-check
```

## 📚 Content Organization

### Documentation Sections

#### Getting Started
- Installation guide
- Quick start tutorial
- Basic configuration
- First project setup

#### CLI Reference
- Command documentation
- Configuration options
- Plugin system overview
- Troubleshooting guides

#### Plugin Development
- Plugin architecture
- Development workflow
- API reference
- Best practices

#### API Documentation
- Core interfaces
- Type definitions
- Method signatures
- Code examples

### Navigation Structure
Configure navigation in `content/meta.json`:

```json
{
  "title": "Getting Started",
  "pages": [
    "installation",
    "quickstart",
    {
      "title": "Configuration",
      "pages": ["basic-config", "advanced-config"]
    }
  ]
}
```

## 📝 Writing Documentation

### MDX with Fumadocs
Create documentation using MDX files in the `content/docs/` directory:

```mdx
---
title: Getting Started with DevEx
description: Learn how to install and configure DevEx for your development environment
---

import { Callout } from 'fumadocs-ui/components/callout'
import { Tabs, Tab } from 'fumadocs-ui/components/tabs'

# Getting Started

Welcome to DevEx! This guide will help you get started quickly.

<Callout type="info">
DevEx supports over 36 package managers and desktop environments.
</Callout>

## Installation

<Tabs items={['Linux', 'macOS', 'Windows']}>
<Tab value="Linux">
```bash
curl -fsSL https://devex.sh/install | bash
```
</Tab>
<Tab value="macOS">
```bash
brew install jameswlane/tap/devex
```
</Tab>
<Tab value="Windows">
```powershell
# Coming soon
```
</Tab>
</Tabs>
```

### Available Components

#### Fumadocs UI Components
- **Callout**: Information, warning, error callouts
- **Tabs**: Tabbed content organization
- **Code Block**: Syntax-highlighted code with copy buttons
- **File Tree**: Visual directory structure
- **Cards**: Content cards and feature highlights

#### Custom Components
```mdx
<Steps>
<Step>Install DevEx CLI</Step>
<Step>Configure your environment</Step>
<Step>Install your first application</Step>
</Steps>

<PluginGrid>
<PluginCard name="apt" type="package-manager" />
<PluginCard name="gnome" type="desktop" />
</PluginGrid>
```

## 🎨 Theming and Styling

### Fumadocs Configuration
```typescript
// app/layout.config.tsx
export const layoutOptions: LayoutOptions = {
  nav: {
    title: 'DevEx',
    url: 'https://devex.sh'
  },
  sidebar: {
    defaultOpenLevel: 1,
    banner: <RootToggle />
  },
  toc: {
    enabled: true,
    component: <TOCItems />
  }
};
```

### Dark/Light Mode
Fumadocs includes built-in theme switching:
- Automatic system preference detection
- Manual theme toggle
- Persistent theme selection
- CSS custom properties for consistent theming

## 🔍 Search Implementation

### Fumadocs Search
Built-in search functionality powered by Fumadocs:

```typescript
// app/api/search/route.ts
import { createSearchAPI } from 'fumadocs-core/search/server'
import { source } from '@/lib/source'

export const { GET } = createSearchAPI('advanced', {
  indexes: source.getPages().map(page => ({
    title: page.data.title,
    structuredData: page.data.structuredData,
    id: page.url,
    url: page.url
  }))
})
```

### Search Features
- **Real-time search**: Instant results as you type
- **Structured data**: Search through headings and content
- **Keyboard shortcuts**: Quick access with Cmd/Ctrl+K
- **Highlighting**: Matched text highlighting in results

## 📱 Mobile Experience

### Responsive Design
- **Mobile-first approach**: Optimized for mobile devices
- **Touch navigation**: Swipe gestures and touch-friendly interface  
- **Collapsible sidebar**: Space-efficient mobile navigation
- **Readable typography**: Optimal font sizes across all devices

### Performance Optimizations
- **Static site generation**: Pre-rendered pages for fast loading
- **Image optimization**: Automatic WebP conversion and sizing
- **Code splitting**: Minimal JavaScript bundles
- **Prefetching**: Smart prefetching for better navigation

## 🚀 Deployment

### Vercel Integration
Automatic deployment configuration:

```json
{
  "buildCommand": "pnpm build",
  "outputDirectory": ".next",
  "framework": "nextjs",
  "functions": {
    "app/api/search/route.ts": {
      "runtime": "nodejs18.x"
    }
  }
}
```

### Environment Configuration
```bash
# Development
NEXT_PUBLIC_SITE_URL=http://localhost:3000
NODE_ENV=development

# Production  
NEXT_PUBLIC_SITE_URL=https://docs.devex.sh
VERCEL_URL=docs.devex.sh
NODE_ENV=production
```

## 🔧 Content Source Configuration

### Source Adapter
```typescript
// lib/source.ts
import { loader } from 'fumadocs-core/source'
import { createMDXSource } from 'fumadocs-mdx'
import { docs, meta } from '@/.source'

export const source = loader({
  baseUrl: '/docs',
  source: createMDXSource(docs, meta)
})
```

### MDX Processing
```typescript
// source.config.ts
export default {
  transformers: [
    // Add syntax highlighting
    transformerNotationDiff(),
    transformerNotationHighlight(),
    // Add copy buttons to code blocks
    transformerCopyButton({
      visibility: 'hover'
    })
  ]
}
```

## 🧪 Development Workflow

### Content Development
```bash
# Start development with hot reload
pnpm dev

# Validate content structure
pnpm lint:content

# Check for broken links
pnpm link-check

# Build and test locally
pnpm build && pnpm start
```

### Quality Checks
```bash
# Type checking
pnpm type-check

# ESLint
pnpm lint

# Format with Prettier
pnpm format

# Spell check (if configured)
pnpm spell-check
```

## 📊 Analytics and Monitoring

### Performance Metrics
- **Core Web Vitals**: LCP, FID, CLS monitoring
- **Page Load Times**: First contentful paint tracking
- **Search Analytics**: Query performance and usage
- **User Engagement**: Page views and reading patterns

### SEO Optimization
- **Meta tags**: Automatic generation from frontmatter
- **Structured data**: JSON-LD for rich snippets
- **Sitemap**: Automatic sitemap generation
- **Open Graph**: Social media preview optimization

## 🤝 Contributing

### Documentation Guidelines
1. **Clear structure**: Use consistent heading hierarchy
2. **Code examples**: Include working, tested examples
3. **Cross-references**: Link related concepts
4. **Accessibility**: Use proper alt text and semantic HTML

### Content Review Process
1. Create feature branch for documentation changes
2. Write/edit MDX files with proper frontmatter
3. Test locally with `pnpm dev`
4. Submit PR with clear description of changes
5. Review for accuracy, clarity, and consistency

## 📄 License

This documentation is licensed under the [Apache-2.0 License](../../LICENSE).

---

<div align="center">

**[Visit Documentation](https://docs.devex.sh)** • **[Edit Content](https://github.com/jameswlane/devex/tree/main/apps/docs/content)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
