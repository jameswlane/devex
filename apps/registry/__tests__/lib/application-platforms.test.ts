import { readFileSync } from "node:fs";
import path from "node:path";
import { beforeEach, describe, expect, it, vi } from "vitest";
import * as yaml from "yaml";

vi.mock("@/lib/redis", () => ({
	redis: {
		get: vi.fn().mockResolvedValue(null),
		set: vi.fn(),
		incr: vi.fn(),
		expire: vi.fn(),
		del: vi.fn(),
		exists: vi.fn(),
	},
}));

vi.mock("@/lib/logger", () => ({
	logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
}));

import {
	type ApplicationYaml,
	applicationPlatformData,
	platformDocuments,
} from "@/lib/application-platforms";
import { transformationService } from "@/lib/transformation-service";

const catalog = path.resolve(__dirname, "../../../cli/config/applications");
const loadApp = (file: string) =>
	yaml.parse(
		readFileSync(path.join(catalog, file), "utf-8"),
	) as ApplicationYaml;

describe("application platforms (#479)", () => {
	beforeEach(() => vi.clearAllMocks());

	it("syncs desktop_environments and each platform document as written", () => {
		const bat = loadApp("development/bat.yaml") as ApplicationYaml & {
			linux: Record<string, unknown>;
		};
		const data = applicationPlatformData(bat);

		expect(data.desktopEnvironments).toEqual(["all"]);
		expect(data.platforms.linux).toEqual(bat.linux);
		expect(data.platforms.linux).toMatchObject({
			install_method: "apt",
			install_command: "bat",
		});
		expect(data).toMatchObject({
			supportsLinux: true,
			supportsMacOS: true,
			supportsWindows: true,
		});
	});

	it("syncs all_platforms under all", () => {
		const data = applicationPlatformData(loadApp("databases/mysql.yaml"));

		expect(data.platforms).toEqual({
			all: { install_method: "docker", install_command: "mysql:8.4" },
		});
		expect(data.supportsLinux && data.supportsMacOS).toBe(true);
	});

	it("serves the full stored document from the registry transformation", async () => {
		const bat = applicationPlatformData(loadApp("development/bat.yaml"));
		// The shape /api/v1/registry passes in: raw rows from the database.
		const [app] = await transformationService.transformApplications([
			{
				name: "bat",
				description: "A cat(1) clone with wings",
				category: "Utilities",
				official: false,
				default: true,
				tags: [],
				desktopEnvironments: bat.desktopEnvironments,
				githubPath: "apps/cli/config/applications/development/bat.yaml",
				platforms: bat.platforms,
			},
		]);

		expect(app.platforms.linux).toEqual(bat.platforms.linux);
		expect(app.platforms.linux?.install_method).toBe("apt");
		expect(app.platforms.macos?.install_command).toBe("bat");
		expect(app.desktopEnvironments).toEqual(["all"]);
		// List and registry responses go through the same function, so
		// transforming an already-converted document changes nothing.
		expect(platformDocuments(app.platforms)).toEqual(app.platforms);
	});

	it("keeps all and fills missing platforms and alternatives", () => {
		expect(platformDocuments({ all: { install_method: "docker" } })).toEqual({
			all: { install_method: "docker", alternatives: [] },
			linux: null,
			macos: null,
			windows: null,
		});
		expect(platformDocuments(null)).toEqual({
			linux: null,
			macos: null,
			windows: null,
		});
	});
});
