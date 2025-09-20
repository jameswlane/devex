# DevEx Website

[![Next.js](https://img.shields.io/badge/Next.js-15.5-black?logo=next.js)](https://nextjs.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.x-blue?logo=typescript)](https://www.typescriptlang.org/)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind%20CSS-3.x-06B6D4?logo=tailwindcss)](https://tailwindcss.com/)
[![License](https://img.shields.io/github/license/jameswlane/devex)](../../LICENSE)
[![Vercel](https://img.shields.io/badge/Deployed%20on-Vercel-black?logo=vercel)](https://devex.sh)

Modern, responsive marketing website for DevEx built with Next.js 15, TypeScript, and Tailwind CSS. Hosted at [devex.sh](https://devex.sh).

## 🚀 Features

- **🎨 Modern Design**: Clean, professional interface with dark/light mode
- **📱 Mobile-First**: Responsive design optimized for all devices
- **⚡ Fast Loading**: Static generation with Next.js optimizations
- **🎯 SEO Optimized**: Meta tags, structured data, and sitemap generation
- **🔍 Plugin Showcase**: Interactive plugin catalog and feature highlights
- **📊 Analytics**: User behavior tracking and conversion optimization

## 🏗️ Architecture

### Website Structure
```
apps/web/
├── src/
│   ├── app/                 # Next.js 15 App Router
│   │   ├── (marketing)/     # Marketing pages route group
│   │   ├── blog/           # Blog posts and articles
│   │   ├── plugins/        # Plugin showcase pages
│   │   └── layout.tsx      # Root layout
│   ├── components/         # Reusable UI components
│   │   ├── Hero/           # Hero sections
│   │   ├── Features/       # Feature highlights
│   │   ├── PluginGrid/     # Plugin showcases
│   │   ├── Navigation/     # Site navigation
│   │   └── Footer/         # Site footer
│   ├── lib/               # Utilities and configurations
│   │   ├── analytics/     # Analytics integration
│   │   ├── seo/          # SEO utilities
│   │   └── api/          # API clients
│   └── styles/           # Global styles and Tailwind config
├── public/              # Static assets
│   ├── images/         # Images and graphics
│   ├── icons/          # Icons and logos
│   └── robots.txt      # Search engine directives
├── content/            # Static content and blog posts
│   ├── blog/          # Blog post markdown files
│   └── case-studies/  # Customer case studies
└── package.json       # Dependencies and scripts
```

### Technology Stack
- **Next.js 15**: React framework with App Router and static generation
- **TypeScript**: Type-safe development
- **Tailwind CSS**: Utility-first styling with custom design system
- **MDX**: Markdown with React components for blog content
- **Vercel**: Hosting, deployment, and edge functions
- **Vercel Analytics**: Privacy-focused website analytics

## 🎨 Design System

### Color Palette
```css
/* Primary Colors */
--primary-50: #eff6ff;
--primary-500: #3b82f6;
--primary-900: #1e3a8a;

/* Semantic Colors */
--success: #10b981;
--warning: #f59e0b;
--error: #ef4444;
--neutral: #6b7280;

/* Dark Mode */
--dark-bg: #0f172a;
--dark-surface: #1e293b;
--dark-text: #f8fafc;
```

### Typography
```css
/* Font Stack */
--font-sans: ui-sans-serif, system-ui, sans-serif;
--font-mono: ui-monospace, 'SF Mono', Consolas, monospace;

/* Type Scale */
--text-xs: 0.75rem;    /* 12px */
--text-sm: 0.875rem;   /* 14px */
--text-base: 1rem;     /* 16px */
--text-lg: 1.125rem;   /* 18px */
--text-xl: 1.25rem;    /* 20px */
--text-2xl: 1.5rem;    /* 24px */
--text-3xl: 1.875rem;  /* 30px */
--text-4xl: 2.25rem;   /* 36px */
```

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

# Static export (if needed)
pnpm export
```

### Environment Configuration
```bash
# Development
NEXT_PUBLIC_SITE_URL=http://localhost:3000
NEXT_PUBLIC_GA_ID=G-XXXXXXXXXX

# Production
NEXT_PUBLIC_SITE_URL=https://devex.sh
NEXT_PUBLIC_GA_ID=G-PRODUCTION-ID
VERCEL_URL=devex.sh
```

## 📄 Page Structure

### Landing Page (`/`)
- **Hero Section**: Main value proposition and CTA
- **Features Grid**: Key DevEx features and benefits
- **Plugin Showcase**: Highlight popular plugins with categories
- **Platform Support**: Visual representation of supported platforms
- **Social Proof**: Testimonials and usage statistics
- **Call-to-Action**: Installation instructions and getting started

### Plugin Showcase (`/plugins`)
- **Plugin Categories**: Browse plugins by type (package managers, desktop environments)
- **Search & Filter**: Find plugins by name, platform, or features
- **Plugin Cards**: Visual plugin information with installation commands
- **Platform Compatibility**: Clear platform support indicators

### Documentation (`/docs`)
- **Quick Start Guide**: Step-by-step installation and setup
- **CLI Reference**: Complete command documentation
- **Plugin Development**: How to create custom plugins
- **API Reference**: Technical documentation for developers

### Blog (`/blog`)
- **Release Notes**: New feature announcements and updates
- **Tutorials**: In-depth guides and best practices
- **Case Studies**: Real-world usage examples
- **Community**: Guest posts and community highlights

## 🔍 SEO Optimization

### Meta Tags
```typescript
export const metadata: Metadata = {
  title: 'DevEx - Streamline Your Development Environment',
  description: 'Enterprise-grade CLI tool for automated development environment setup across Linux, macOS, and Windows. 36+ plugins for package managers and desktop environments.',
  keywords: ['development environment', 'CLI tool', 'package manager', 'Linux', 'macOS', 'automation'],
  authors: [{ name: 'James Lane', url: 'https://github.com/jameswlane' }],
  openGraph: {
    title: 'DevEx - Streamline Your Development Environment',
    description: 'Automate your dev setup with 36+ plugins for package managers and desktop environments.',
    url: 'https://devex.sh',
    siteName: 'DevEx',
    images: [
      {
        url: 'https://devex.sh/og-image.png',
        width: 1200,
        height: 630,
        alt: 'DevEx - Development Environment Automation'
      }
    ]
  },
  twitter: {
    card: 'summary_large_image',
    title: 'DevEx - Streamline Your Development Environment',
    description: 'Enterprise-grade CLI for dev environment automation',
    images: ['https://devex.sh/twitter-image.png']
  }
}
```

### Structured Data
```typescript
const structuredData = {
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  "name": "DevEx",
  "description": "Enterprise-grade CLI tool for development environment setup",
  "url": "https://devex.sh",
  "downloadUrl": "https://github.com/jameswlane/devex/releases",
  "applicationCategory": "DeveloperApplication",
  "operatingSystem": ["Linux", "macOS", "Windows"],
  "offers": {
    "@type": "Offer",
    "price": "0",
    "priceCurrency": "USD"
  }
}
```

## 📊 Analytics and Monitoring

### Performance Tracking
- **Core Web Vitals**: LCP, FID, CLS monitoring
- **Page Load Times**: First contentful paint and time to interactive
- **User Journey**: Conversion funnel from landing to installation
- **A/B Testing**: Feature and content optimization

### User Analytics
```typescript
// Track key user actions
trackEvent('plugin_interest', {
  plugin_name: 'package-manager-apt',
  plugin_category: 'package-manager',
  user_platform: 'linux'
})

trackEvent('installation_started', {
  source: 'hero_cta',
  method: 'curl_script'
})
```

### Conversion Metrics
- **Installation Rate**: Visitors who download/install DevEx
- **Plugin Discovery**: Most viewed plugin categories
- **Documentation Engagement**: Time spent on docs pages
- **Support Requests**: Common user questions and issues

## 🎨 Component Library

### Hero Components
```tsx
interface HeroProps {
  title: string
  subtitle: string
  ctaText: string
  ctaLink: string
  backgroundImage?: string
  features?: string[]
}

export function Hero({ title, subtitle, ctaText, ctaLink }: HeroProps) {
  return (
    <section className="bg-gradient-to-br from-blue-600 to-purple-700">
      <div className="container mx-auto px-4 py-20">
        <h1 className="text-4xl md:text-6xl font-bold text-white mb-6">
          {title}
        </h1>
        <p className="text-xl text-blue-100 mb-8 max-w-2xl">
          {subtitle}
        </p>
        <Button href={ctaLink} size="large" variant="secondary">
          {ctaText}
        </Button>
      </div>
    </section>
  )
}
```

### Plugin Grid
```tsx
interface PluginGridProps {
  plugins: Plugin[]
  category?: string
  limit?: number
}

export function PluginGrid({ plugins, category, limit }: PluginGridProps) {
  const filteredPlugins = category 
    ? plugins.filter(p => p.category === category)
    : plugins
  
  const displayPlugins = limit 
    ? filteredPlugins.slice(0, limit)
    : filteredPlugins

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      {displayPlugins.map(plugin => (
        <PluginCard key={plugin.id} plugin={plugin} />
      ))}
    </div>
  )
}
```

### Feature Cards
```tsx
interface FeatureProps {
  icon: React.ComponentType
  title: string
  description: string
  link?: string
}

export function FeatureCard({ icon: Icon, title, description, link }: FeatureProps) {
  return (
    <div className="bg-white rounded-lg p-6 shadow-sm hover:shadow-md transition-shadow">
      <Icon className="w-12 h-12 text-blue-600 mb-4" />
      <h3 className="text-xl font-semibold mb-2">{title}</h3>
      <p className="text-gray-600 mb-4">{description}</p>
      {link && (
        <Link href={link} className="text-blue-600 hover:text-blue-700 font-medium">
          Learn more →
        </Link>
      )}
    </div>
  )
}
```

## 📱 Mobile Experience

### Responsive Design
- **Mobile-First**: Designed for mobile, enhanced for desktop
- **Touch-Friendly**: Large touch targets and intuitive gestures
- **Fast Loading**: Optimized images and minimal JavaScript
- **Progressive Enhancement**: Works without JavaScript

### Performance Optimizations
- **Image Optimization**: Next.js automatic image optimization
- **Code Splitting**: Page-based and component-based splitting
- **Static Generation**: Pre-rendered pages for fast loading
- **CDN Delivery**: Global content delivery via Vercel Edge Network

## 🚀 Deployment

### Vercel Configuration
```json
{
  "buildCommand": "pnpm build",
  "outputDirectory": ".next",
  "framework": "nextjs",
  "functions": {
    "src/app/api/**": {
      "runtime": "nodejs18.x"
    }
  },
  "redirects": [
    {
      "source": "/download",
      "destination": "https://github.com/jameswlane/devex/releases/latest"
    }
  ]
}
```

### Environment Variables
```bash
# Analytics
NEXT_PUBLIC_GA_ID=G-XXXXXXXXXX
NEXT_PUBLIC_PLAUSIBLE_DOMAIN=devex.sh

# API Integration
NEXT_PUBLIC_REGISTRY_URL=https://registry.devex.sh
NEXT_PUBLIC_DOCS_URL=https://docs.devex.sh

# Feature Flags
NEXT_PUBLIC_BLOG_ENABLED=true
NEXT_PUBLIC_NEWSLETTER_ENABLED=true
```

## 🎯 Content Strategy

### Blog Content
- **Release Announcements**: New features and plugin releases
- **Technical Tutorials**: Advanced DevEx usage and customization
- **Platform Guides**: Platform-specific installation and configuration
- **Community Spotlights**: User success stories and contributions

### Landing Page Optimization
- **Clear Value Proposition**: Immediate understanding of DevEx benefits
- **Social Proof**: Usage statistics, testimonials, GitHub stars
- **Multiple CTAs**: Different entry points for different user types
- **Feature Comparison**: How DevEx compares to alternatives

## 🧪 Testing and Quality

### Performance Testing
```bash
# Lighthouse CI
pnpm lighthouse

# Bundle analysis
pnpm analyze

# Performance monitoring
pnpm test:performance
```

### A/B Testing
```typescript
// Feature flag integration
import { useFeatureFlag } from '@/lib/feature-flags'

export function Hero() {
  const showNewCTA = useFeatureFlag('new-cta-design')
  
  return (
    <section>
      {/* Hero content */}
      {showNewCTA ? <NewCTAButton /> : <OriginalCTAButton />}
    </section>
  )
}
```

## 🤝 Contributing

### Content Updates
1. Fork the repository
2. Create content branch: `git checkout -b content/new-blog-post`
3. Add/edit content in `content/` directory
4. Test locally: `pnpm dev`
5. Submit Pull Request

### Design Changes
1. Follow existing design system patterns
2. Test on multiple screen sizes
3. Verify accessibility compliance
4. Check performance impact
5. Update component documentation

## 📄 License

The DevEx Website is licensed under the [Apache-2.0 License](../../LICENSE).

---

<div align="center">

**[Visit Website](https://devex.sh)** • **[View Source](https://github.com/jameswlane/devex/tree/main/apps/web)** • **[Report Issues](https://github.com/jameswlane/devex/issues)**

</div>
