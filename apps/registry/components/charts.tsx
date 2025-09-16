'use client'

import { 
  PieChart, 
  Pie, 
  Cell, 
  BarChart, 
  Bar, 
  XAxis, 
  YAxis, 
  CartesianGrid, 
  Tooltip, 
  ResponsiveContainer,
  LineChart,
  Line,
  Legend
} from 'recharts'

interface ChartData {
  name: string
  value: number
  color?: string
}

interface PlatformData {
  platform: string
  count: number
}

interface CategoryData {
  category: string
  applications: number
  plugins: number
  configs: number
}

// Color palette for charts
const COLORS = {
  primary: ['#3B82F6', '#1D4ED8', '#1E40AF', '#1E3A8A'],
  platforms: {
    linux: '#EF4444',
    macos: '#6B7280',
    windows: '#3B82F6'
  },
  categories: [
    '#3B82F6', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6',
    '#06B6D4', '#84CC16', '#F97316', '#EC4899', '#6366F1'
  ]
}

interface PlatformDistributionChartProps {
  data: PlatformData[]
  isLoading?: boolean
}

export function PlatformDistributionChart({ data, isLoading }: PlatformDistributionChartProps) {
  if (isLoading) {
    return (
      <div className="bg-white rounded-lg shadow-md p-6">
        <h3 className="text-lg font-semibold text-gray-800 mb-4">Platform Distribution</h3>
        <div className="h-64 flex items-center justify-center">
          <div className="animate-pulse">
            <div className="w-48 h-48 bg-gray-200 rounded-full"></div>
          </div>
        </div>
      </div>
    )
  }

  const chartData = data.map((item, index) => ({
    name: item.platform.charAt(0).toUpperCase() + item.platform.slice(1),
    value: item.count,
    color: COLORS.platforms[item.platform as keyof typeof COLORS.platforms] || COLORS.categories[index % COLORS.categories.length]
  }))

  return (
    <div className="bg-white rounded-lg shadow-md p-6">
      <h3 className="text-lg font-semibold text-gray-800 mb-4">Platform Distribution</h3>
      <ResponsiveContainer width="100%" height={300}>
        <PieChart>
          <Pie
            data={chartData}
            cx="50%"
            cy="50%"
            labelLine={false}
            label={({ name, percent }: any) => `${name} ${(percent * 100).toFixed(0)}%`}
            outerRadius={80}
            fill="#8884d8"
            dataKey="value"
          >
            {chartData.map((entry, index) => (
              <Cell key={`cell-${index}`} fill={entry.color} />
            ))}
          </Pie>
          <Tooltip formatter={(value) => [`${value} applications`, 'Count']} />
        </PieChart>
      </ResponsiveContainer>
    </div>
  )
}

interface CategoryComparisonChartProps {
  applicationsData: Record<string, number>
  pluginsData: Record<string, number>
  configsData?: Record<string, number>
  isLoading?: boolean
}

export function CategoryComparisonChart({ 
  applicationsData, 
  pluginsData, 
  configsData = {},
  isLoading 
}: CategoryComparisonChartProps) {
  if (isLoading) {
    return (
      <div className="bg-white rounded-lg shadow-md p-6">
        <h3 className="text-lg font-semibold text-gray-800 mb-4">Category Comparison</h3>
        <div className="h-80 flex items-center justify-center">
          <div className="animate-pulse space-y-3 w-full">
            <div className="grid grid-cols-3 gap-4">
              <div className="h-4 bg-gray-200 rounded"></div>
              <div className="h-4 bg-gray-200 rounded"></div>
              <div className="h-4 bg-gray-200 rounded"></div>
            </div>
            <div className="space-y-2">
              {Array.from({ length: 6 }).map((_, i) => (
                <div key={i} className="h-8 bg-gray-200 rounded"></div>
              ))}
            </div>
          </div>
        </div>
      </div>
    )
  }

  // Combine all categories and create chart data
  const allCategories = new Set([
    ...Object.keys(applicationsData),
    ...Object.keys(pluginsData),
    ...Object.keys(configsData)
  ])

  const chartData: CategoryData[] = Array.from(allCategories)
    .map(category => ({
      category: category.charAt(0).toUpperCase() + category.slice(1).replace('-', ' '),
      applications: applicationsData[category] || 0,
      plugins: pluginsData[category] || 0,
      configs: configsData[category] || 0,
    }))
    .sort((a, b) => (b.applications + b.plugins + b.configs) - (a.applications + a.plugins + a.configs))
    .slice(0, 8) // Top 8 categories

  return (
    <div className="bg-white rounded-lg shadow-md p-6">
      <h3 className="text-lg font-semibold text-gray-800 mb-4">Category Comparison</h3>
      <ResponsiveContainer width="100%" height={350}>
        <BarChart
          data={chartData}
          margin={{
            top: 20,
            right: 30,
            left: 20,
            bottom: 60,
          }}
        >
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis 
            dataKey="category" 
            angle={-45}
            textAnchor="end"
            height={80}
            fontSize={12}
          />
          <YAxis />
          <Tooltip />
          <Legend />
          <Bar dataKey="applications" stackId="a" fill={COLORS.primary[0]} name="Applications" />
          <Bar dataKey="plugins" stackId="a" fill={COLORS.primary[1]} name="Plugins" />
          {Object.keys(configsData).length > 0 && (
            <Bar dataKey="configs" stackId="a" fill={COLORS.primary[2]} name="Configs" />
          )}
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}

interface RegistryOverviewChartProps {
  totals: {
    applications: number
    plugins: number
    configs: number
    stacks: number
  }
  isLoading?: boolean
}

export function RegistryOverviewChart({ totals, isLoading }: RegistryOverviewChartProps) {
  if (isLoading) {
    return (
      <div className="bg-white rounded-lg shadow-md p-6">
        <h3 className="text-lg font-semibold text-gray-800 mb-4">Registry Overview</h3>
        <div className="h-64 flex items-center justify-center">
          <div className="animate-pulse">
            <div className="w-48 h-48 bg-gray-200 rounded-full"></div>
          </div>
        </div>
      </div>
    )
  }

  const chartData = [
    { name: 'Applications', value: totals.applications, color: COLORS.primary[0] },
    { name: 'Plugins', value: totals.plugins, color: COLORS.primary[1] },
    { name: 'Configs', value: totals.configs, color: COLORS.primary[2] },
    { name: 'Stacks', value: totals.stacks, color: COLORS.primary[3] },
  ].filter(item => item.value > 0) // Only show categories with data

  return (
    <div className="bg-white rounded-lg shadow-md p-6">
      <h3 className="text-lg font-semibold text-gray-800 mb-4">Registry Overview</h3>
      <ResponsiveContainer width="100%" height={300}>
        <PieChart>
          <Pie
            data={chartData}
            cx="50%"
            cy="50%"
            labelLine={false}
            label={({ name, value, percent }: any) => `${name}: ${value} (${(percent * 100).toFixed(1)}%)`}
            outerRadius={100}
            fill="#8884d8"
            dataKey="value"
          >
            {chartData.map((entry, index) => (
              <Cell key={`cell-${index}`} fill={entry.color} />
            ))}
          </Pie>
          <Tooltip formatter={(value, name) => [`${value}`, name]} />
        </PieChart>
      </ResponsiveContainer>
    </div>
  )
}

// Growth trend chart (placeholder for future use)
interface GrowthTrendChartProps {
  data?: Array<{ date: string; applications: number; plugins: number }>
  isLoading?: boolean
}

export function GrowthTrendChart({ data = [], isLoading }: GrowthTrendChartProps) {
  if (isLoading) {
    return (
      <div className="bg-white rounded-lg shadow-md p-6">
        <h3 className="text-lg font-semibold text-gray-800 mb-4">Growth Trends</h3>
        <div className="h-64 flex items-center justify-center">
          <div className="animate-pulse space-y-2 w-full">
            {Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className="h-6 bg-gray-200 rounded"></div>
            ))}
          </div>
        </div>
      </div>
    )
  }

  if (data.length === 0) {
    return (
      <div className="bg-white rounded-lg shadow-md p-6">
        <h3 className="text-lg font-semibold text-gray-800 mb-4">Growth Trends</h3>
        <div className="h-64 flex items-center justify-center">
          <div className="text-center">
            <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
            </svg>
            <h3 className="mt-2 text-sm font-medium text-gray-900">No trend data available</h3>
            <p className="mt-1 text-sm text-gray-500">Historical data will appear here as the registry grows.</p>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="bg-white rounded-lg shadow-md p-6">
      <h3 className="text-lg font-semibold text-gray-800 mb-4">Growth Trends</h3>
      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="date" />
          <YAxis />
          <Tooltip />
          <Legend />
          <Line 
            type="monotone" 
            dataKey="applications" 
            stroke={COLORS.primary[0]} 
            strokeWidth={2}
            name="Applications"
          />
          <Line 
            type="monotone" 
            dataKey="plugins" 
            stroke={COLORS.primary[1]} 
            strokeWidth={2}
            name="Plugins"
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}

// Mini chart components for stat cards
interface MiniChartProps {
  data: number[]
  color?: string
  height?: number
}

export function MiniLineChart({ data, color = COLORS.primary[0], height = 40 }: MiniChartProps) {
  const chartData = data.map((value, index) => ({ index, value }))
  
  return (
    <ResponsiveContainer width="100%" height={height}>
      <LineChart data={chartData}>
        <Line 
          type="monotone" 
          dataKey="value" 
          stroke={color} 
          strokeWidth={2}
          dot={false}
        />
      </LineChart>
    </ResponsiveContainer>
  )
}

export function MiniBarChart({ data, color = COLORS.primary[0], height = 40 }: MiniChartProps) {
  const chartData = data.map((value, index) => ({ index, value }))
  
  return (
    <ResponsiveContainer width="100%" height={height}>
      <BarChart data={chartData}>
        <Bar dataKey="value" fill={color} />
      </BarChart>
    </ResponsiveContainer>
  )
}