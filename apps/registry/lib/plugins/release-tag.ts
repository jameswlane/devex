/** The tag releases are cut under: "packages/<plugin>/v<version>". */
export function pluginReleaseTag(pluginName: string, version: string): string {
	return `packages/${pluginName}/v${version}`;
}
