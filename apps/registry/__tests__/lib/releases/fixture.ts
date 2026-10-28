import type { ReleasePayload } from "@/lib/releases/contract";

export const payload: ReleasePayload = {
	plugin: "package-manager-apt",
	version: "1.2.3",
	tag: "packages/package-manager-apt/v1.2.3",
	prerelease: false,
	publishedAt: "2026-10-27T00:00:00.000Z",
	metadata: {
		description: "APT package manager",
		type: "package-manager",
		platforms: ["linux"],
		dependencies: [],
		conflicts: [],
	},
	binaries: {
		"linux-amd64": {
			url: "https://example.com/apt.tar.gz",
			checksum: "a".repeat(64),
			size: 123,
			format: "tar.gz",
			archiveEntry: "apt",
		},
	},
};
