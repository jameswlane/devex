export interface ApplicationWhereInput {
	category?: { contains: string; mode: "insensitive" };
	linuxSupportId?: { not: null };
	macosSupportId?: { not: null };
	windowsSupportId?: { not: null };
	OR?: Array<{
		name?: { contains: string; mode: "insensitive" };
		description?: { contains: string; mode: "insensitive" };
		tags?: { has: string };
	}>;
}

export interface PluginWhereInput {
	type?: { contains: string; mode: "insensitive" };
	status?: string;
	OR?: Array<{
		name?: { contains: string; mode: "insensitive" };
		description?: { contains: string; mode: "insensitive" };
	}>;
}

export interface ConfigWhereInput {
	category?: { contains: string; mode: "insensitive" };
	type?: { contains: string; mode: "insensitive" };
	OR?: Array<{
		name?: { contains: string; mode: "insensitive" };
		description?: { contains: string; mode: "insensitive" };
	}>;
}

export interface StackWhereInput {
	category?: { contains: string; mode: "insensitive" };
	OR?: Array<{
		name?: { contains: string; mode: "insensitive" };
		description?: { contains: string; mode: "insensitive" };
	}>;
}
