import type { PluginResponse } from '@/lib/types/registry'

interface PluginCardProps {
  plugin: PluginResponse
}

export function PluginCard({ plugin }: PluginCardProps) {
  const getStatusColor = (status: string) => {
    switch (status) {
      case 'active':
        return 'bg-green-100 text-green-800'
      case 'deprecated':
        return 'bg-red-100 text-red-800'
      case 'experimental':
        return 'bg-yellow-100 text-yellow-800'
      default:
        return 'bg-gray-100 text-gray-800'
    }
  }

  const getTypeColor = (type: string) => {
    const colors = [
      'bg-blue-100 text-blue-800',
      'bg-purple-100 text-purple-800',
      'bg-indigo-100 text-indigo-800',
      'bg-pink-100 text-pink-800',
      'bg-green-100 text-green-800',
      'bg-yellow-100 text-yellow-800',
      'bg-red-100 text-red-800',
      'bg-gray-100 text-gray-800'
    ]
    const index = Math.abs(type.split('').reduce((a, b) => a + b.charCodeAt(0), 0)) % colors.length
    return colors[index]
  }

  return (
    <div className="bg-white rounded-lg shadow-md p-6 hover:shadow-lg transition-shadow">
      {/* Header */}
      <div className="flex items-start justify-between mb-3">
        <h3 className="text-lg font-semibold text-gray-900">{plugin.name}</h3>
        <div className="flex items-center space-x-1">
          <span className={`text-xs px-2 py-1 rounded-sm ${getStatusColor(plugin.status)}`}>
            {plugin.status}
          </span>
        </div>
      </div>

      {/* Description */}
      <p className="text-gray-600 text-sm mb-4 line-clamp-3">{plugin.description}</p>

      {/* Type & Priority */}
      <div className="mb-4">
        <span className={`inline-block text-xs px-2 py-1 rounded mr-2 ${getTypeColor(plugin.type)}`}>
          {plugin.type.split('-').map(word => word.charAt(0).toUpperCase() + word.slice(1)).join(' ')}
        </span>
        <span className="text-xs text-gray-500">
          Priority: {plugin.priority}
        </span>
      </div>

      {/* Platform Support */}
      <div className="mb-4">
        <div className="text-xs text-gray-500 mb-1">Platforms:</div>
        <div className="flex flex-wrap gap-1">
          {plugin.platforms.map((platform) => (
            <span key={platform} className="bg-gray-100 text-gray-700 text-xs px-2 py-1 rounded">
              {platform}
            </span>
          ))}
        </div>
      </div>

      {/* Download Stats */}
      {plugin.downloadCount > 0 && (
        <div className="mb-4 text-sm text-gray-600">
          <div className="flex items-center">
            <svg className="w-4 h-4 mr-1" fill="currentColor" viewBox="0 0 20 20">
              <path fillRule="evenodd" d="M3 17a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm3.293-7.707a1 1 0 011.414 0L9 10.586V3a1 1 0 112 0v7.586l1.293-1.293a1 1 0 111.414 1.414l-3 3a1 1 0 01-1.414 0l-3-3a1 1 0 010-1.414z" clipRule="evenodd" />
            </svg>
            {plugin.downloadCount.toLocaleString()} downloads
          </div>
          {plugin.lastDownload && (
            <div className="text-xs text-gray-500 mt-1">
              Last download: {new Date(plugin.lastDownload).toLocaleDateString()}
            </div>
          )}
        </div>
      )}

      {/* Capabilities */}
      {Object.keys(plugin.supports).length > 0 && (
        <div className="mb-4">
          <div className="text-xs text-gray-500 mb-1">Capabilities:</div>
          <div className="flex flex-wrap gap-1">
            {Object.entries(plugin.supports).slice(0, 3).map(([key, value]) => (
              <span key={key} className="bg-blue-50 text-blue-700 text-xs px-2 py-1 rounded">
                {key}: {String(value)}
              </span>
            ))}
            {Object.keys(plugin.supports).length > 3 && (
              <span className="text-xs text-gray-500">
                +{Object.keys(plugin.supports).length - 3} more
              </span>
            )}
          </div>
        </div>
      )}

      {/* GitHub Link */}
      {plugin.repository && plugin.githubPath && (
        <div className="pt-4 border-t">
          <a
            href={`${plugin.repository}/tree/main/${plugin.githubPath}`}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center text-sm text-blue-600 hover:text-blue-800"
          >
            <svg className="w-4 h-4 mr-1" fill="currentColor" viewBox="0 0 20 20">
              <path fillRule="evenodd" d="M10 0C4.477 0 0 4.484 0 10.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0110 4.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.203 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.942.359.31.678.921.678 1.856 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0020 10.017C20 4.484 15.522 0 10 0z" clipRule="evenodd" />
            </svg>
            View Source
          </a>
        </div>
      )}
    </div>
  )
}