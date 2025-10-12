import { PlatformIcon } from './platform-icon'
import type { ApplicationResponse } from '@/lib/types/registry'

interface ApplicationCardProps {
  app: ApplicationResponse
}

export function ApplicationCard({ app }: ApplicationCardProps) {
  return (
    <div className="bg-white rounded-lg shadow-md p-6 hover:shadow-lg transition-shadow">
      {/* Header */}
      <div className="flex items-start justify-between mb-3">
        <h3 className="text-lg font-semibold text-gray-900">{app.name}</h3>
        <div className="flex items-center space-x-1">
          {app.official && (
            <span className="bg-blue-100 text-blue-800 text-xs px-2 py-1 rounded">
              Official
            </span>
          )}
          {app.default && (
            <span className="bg-green-100 text-green-800 text-xs px-2 py-1 rounded">
              Default
            </span>
          )}
        </div>
      </div>

      {/* Description */}
      <p className="text-gray-600 text-sm mb-4 line-clamp-3">{app.description}</p>

      {/* Category & Tags */}
      <div className="mb-4">
        <span className="inline-block bg-gray-100 text-gray-800 text-xs px-2 py-1 rounded mr-2">
          {app.category}
        </span>
        {app.tags.slice(0, 2).map((tag) => (
          <span key={tag} className="inline-block bg-gray-50 text-gray-600 text-xs px-2 py-1 rounded mr-1">
            {tag}
          </span>
        ))}
        {app.tags.length > 2 && (
          <span className="text-xs text-gray-500">+{app.tags.length - 2} more</span>
        )}
      </div>

      {/* Platform Support */}
      <div className="flex items-center space-x-3 mb-4">
        {app.platforms.linux && (
          <PlatformIcon platform="linux" showLabel />
        )}
        {app.platforms.macos && (
          <PlatformIcon platform="macos" showLabel />
        )}
        {app.platforms.windows && (
          <PlatformIcon platform="windows" showLabel />
        )}
      </div>

      {/* GitHub Link */}
      {app.githubUrl && app.githubPath && (
        <div className="pt-4 border-t">
          <a
            href={`${app.githubUrl}/tree/main/${app.githubPath}`}
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