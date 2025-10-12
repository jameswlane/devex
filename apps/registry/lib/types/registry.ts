// Core type definitions for the registry API

// Alternative installation method
export interface InstallAlternative {
	method: string;
	command: string;
}

// Platform-specific installation info
export interface PlatformInstallInfo {
	installMethod: string;
	installCommand: string;
	officialSupport: boolean;
	alternatives: InstallAlternative[];
}

// Binary information for a specific platform
export interface PlatformBinary {
	url: string;
	checksum: string;
	size: number;
}

// Plugin response type
export interface PluginResponse {
	name: string;
	description: string;
	type: string;
	priority: number;
	status: string;
	supports: Record<string, boolean>;
	platforms: string[];
	tags: string[];
	version: string;
	latestVersion?: string | null;
	author?: string | null;
	license?: string | null;
	homepage?: string | null;
	repository?: string | null;
	dependencies: string[];
	conflicts: string[];
	binaries: Record<string, PlatformBinary>;
	sdkVersion?: string | null;
	apiVersion?: string | null;
	release_tag: string;
	githubPath?: string | null;
	downloadCount: number;
	lastDownload?: string | null;
}

// Application response type
export interface ApplicationResponse {
	name: string;
	description: string;
	category: string;
	type: "application";
	official: boolean;
	default: boolean;
	platforms: {
		linux: PlatformInstallInfo | null;
		macos: PlatformInstallInfo | null;
		windows: PlatformInstallInfo | null;
	};
	tags: string[];
	desktopEnvironments: string[];
	githubUrl?: string | null;
	githubPath?: string | null;
}

// Config response type
export interface ConfigResponse {
	name: string;
	description: string;
	category: string;
	type: string;
	platforms: string[];
	content: unknown; // JSON content can be any structure
	schema?: unknown | null; // JSON schema can be any structure
	githubPath?: string | null;
	downloadCount: number;
	lastDownload?: string | null;
}

// Stack response type
export interface StackResponse {
	name: string;
	description: string;
	category: string;
	applications: string[];
	configs: string[];
	plugins: string[];
	platforms: string[];
	desktopEnvironments: string[];
	prerequisites: unknown; // JSON content
	githubPath?: string | null;
	downloadCount: number;
	lastDownload?: string | null;
}

// Statistics type
export interface RegistryStats {
	total: {
		applications: number;
		plugins: number;
		configs: number;
		stacks: number;
		all: number;
	};
	platforms: {
		linux: number;
		macos: number;
		windows: number;
	};
	activity: {
		totalDownloads: number;
		dailyDownloads: number;
	};
	lastUpdated: string;
}

// Pagination metadata
export interface PaginationMeta {
	page: number;
	limit: number;
	totalPages: number;
	totalItems: {
		plugins: number;
		applications: number;
		configs: number;
		stacks: number;
	};
}

// Main API response type with pagination
export interface PaginatedResponse {
	base_url: string;
	version: string;
	last_updated: string;
	source: string;
	github_url: string;
	data: {
		plugins: PluginResponse[];
		applications: ApplicationResponse[];
		configs: ConfigResponse[];
		stacks: StackResponse[];
	};
	pagination: PaginationMeta;
	stats: RegistryStats;
}
