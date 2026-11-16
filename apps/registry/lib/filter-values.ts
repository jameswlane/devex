import { prisma } from "@/lib/prisma";

// Categories and plugin types are free-form strings written by the catalog
// sync, so the allowed filter values are whatever is stored, not a fixed list.

// Distinct values, case-insensitively, keeping the first spelling in sort order
// ("Dependency" over "dependency"), sorted for display.
export function distinctFilterValues(values: Iterable<string>): string[] {
	const byKey = new Map<string, string>();
	for (const value of [...values].sort()) {
		const trimmed = value.trim();
		if (!trimmed) continue;
		const key = trimmed.toLowerCase();
		if (!byKey.has(key)) byKey.set(key, trimmed);
	}
	return [...byKey.values()].sort((a, b) =>
		a.localeCompare(b, "en", { sensitivity: "base" }),
	);
}

// The allowed value that matches the input case-insensitively, or undefined.
export function matchFilterValue(
	input: string | null | undefined,
	allowed: readonly string[],
): string | undefined {
	const key = input?.trim().toLowerCase();
	if (!key) return undefined;
	return allowed.find((value) => value.toLowerCase() === key);
}

export async function listApplicationCategories(): Promise<string[]> {
	const rows = await prisma.application.findMany({
		distinct: ["category"],
		select: { category: true },
	});
	return distinctFilterValues(rows.map((row) => row.category));
}

export async function listPluginTypes(): Promise<string[]> {
	const rows = await prisma.plugin.findMany({
		distinct: ["type"],
		select: { type: true },
	});
	return distinctFilterValues(rows.map((row) => row.type));
}
