interface LoadingSpinnerProps {
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

export default function LoadingSpinner({ size = 'md', className = '' }: LoadingSpinnerProps) {
  const sizeClasses = {
    sm: 'h-4 w-4',
    md: 'h-8 w-8',
    lg: 'h-12 w-12'
  }

  return (
    <div className={`animate-spin rounded-full border-b-2 border-blue-600 ${sizeClasses[size]} ${className}`} />
  )
}

interface LoadingStateProps {
  message?: string
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

export function LoadingState({ 
  message = 'Loading...', 
  size = 'md', 
  className = '' 
}: LoadingStateProps) {
  return (
    <div className={`flex flex-col items-center justify-center py-8 ${className}`}>
      <LoadingSpinner size={size} className="mb-4" />
      <p className="text-gray-600 text-center">{message}</p>
      <div className="mt-2 flex space-x-1">
        <div className="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style={{ animationDelay: '0ms' }}></div>
        <div className="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style={{ animationDelay: '150ms' }}></div>
        <div className="w-2 h-2 bg-blue-500 rounded-full animate-bounce" style={{ animationDelay: '300ms' }}></div>
      </div>
    </div>
  )
}

interface SkeletonProps {
  className?: string
  count?: number
}

export function Skeleton({ className = 'h-4 w-full', count = 1 }: SkeletonProps) {
  return (
    <>
      {Array.from({ length: count }).map((_, index) => (
        <div
          key={index}
          className={`animate-pulse bg-gray-200 rounded ${className}`}
        />
      ))}
    </>
  )
}

interface CardSkeletonProps {
  className?: string
}

export function CardSkeleton({ className = '' }: CardSkeletonProps) {
  return (
    <div className={`bg-white rounded-lg shadow-md p-6 ${className}`}>
      <Skeleton className="h-6 w-32 mb-4" />
      <Skeleton className="h-8 w-16 mb-2" />
      <Skeleton className="h-4 w-48" />
    </div>
  )
}