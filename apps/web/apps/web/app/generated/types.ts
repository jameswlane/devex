// Generated file - do not edit manually
// Last generated: 2025-09-11T22:22:20.167Z

export interface PlatformInfo {
	installMethod: string;
	installCommand: string;
	alternatives?: Array<{
		install_method: string;
		install_command: string;
		official_support?: boolean;
	}>;
	officialSupport: boolean;
}

export interface Tool {
	name: string;
	description: string;
	category: string;
	type: "application" | "plugin";
	official: boolean;
	default: boolean;
	platforms: {
		linux?: PlatformInfo | null;
		macos?: PlatformInfo | null;
		windows?: PlatformInfo | null;
	};
	tags: string[];
	desktopEnvironments?: string[];

	// Plugin-specific fields
	pluginType?: string;
	priority?: number;
	supports?: Record<string, boolean>;
	status?: string;
}

export interface ToolsData {
	tools: Tool[];
	categories: string[];
	stats: {
		total: number;
		applications: number;
		plugins: number;
		platforms: {
			linux: number;
			macos: number;
			windows: number;
			total: number;
		};
	};
	generated: string;
}
