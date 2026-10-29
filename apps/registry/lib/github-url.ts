export function isGitHubUrl(value: string): boolean {
	try {
		const url = new URL(value);
		return url.protocol === "https:" && url.hostname === "github.com";
	} catch {
		return false;
	}
}
