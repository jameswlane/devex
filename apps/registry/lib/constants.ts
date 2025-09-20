export const APPLICATION_CATEGORIES = [
  'development',
  'productivity',
  'design',
  'communication',
  'media',
  'games',
  'utilities',
  'education',
  'security',
  'databases'
]

export const PLATFORMS = [
  { value: 'linux', label: 'Linux' },
  { value: 'macos', label: 'macOS' },
  { value: 'windows', label: 'Windows' }
]

export const PLUGIN_TYPES = [
  'package-manager',
  'installer',
  'utility',
  'development',
  'configuration',
  'security',
  'monitoring',
  'automation'
]

export const PLUGIN_STATUS = [
  'active',
  'deprecated',
  'experimental'
]

export const PAGE_SIZE_OPTIONS = [10, 20, 50, 100]

export const DEFAULT_PAGE_SIZE = 20

export const MAX_PAGE_SIZE = 100

export const MIN_PAGE_SIZE = 1

export const SEARCH_DEBOUNCE_MS = 500

export const GITHUB_BASE_URL = process.env.GITHUB_BASE_URL || 'https://github.com/jameswlane/devex'