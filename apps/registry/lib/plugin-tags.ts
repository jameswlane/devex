// Tags for a plugin type, served by registry.json and registry-stream.
// Each tag appears once, even when the type is also a semantic tag (#520).
export function extractTagsFromType(type: string): string[] {
	const tags = [type];

	if (type.includes("package-manager")) {
		tags.push("installer", "package-manager");
	}
	if (type.includes("tool")) {
		tags.push("utility", "tool");
	}
	if (type.includes("system")) {
		tags.push("system", "configuration");
	}

	return [...new Set(tags)];
}
