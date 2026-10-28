export { findPluginByNameOrShortName } from "@/lib/plugins/find-plugin";

const SEMVER_PATTERN =
	/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/;

export function normalizePluginVersion(value: string): string | null {
	if (value === "latest") return value;

	const version = value.startsWith("v") ? value.slice(1) : value;
	return SEMVER_PATTERN.test(version) ? version : null;
}
