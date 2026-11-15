import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
	resolve: {
		alias: {
			"@": fileURLToPath(new URL(".", import.meta.url)),
		},
	},
	oxc: {
		jsx: { runtime: "automatic" },
	},
	test: {
		environment: "node",
		setupFiles: ["./vitest.setup.ts"],
		include: ["**/*.{test,spec}.{ts,tsx,js,jsx}"],
		exclude: ["node_modules/**", ".next/**", "generated/**"],
		testTimeout: 10000,
		coverage: {
			provider: "v8",
			include: ["app/**/*.ts", "lib/**/*.ts"],
			exclude: ["lib/types/**/*.ts", "**/*.d.ts", "**/*.{test,spec}.ts"],
			reporter: ["text", "lcov", "html"],
			reportsDirectory: "coverage",
		},
	},
});
