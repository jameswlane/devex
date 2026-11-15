import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
	resolve: {
		alias: {
			"@": fileURLToPath(new URL("./app", import.meta.url)),
		},
	},
	oxc: {
		jsx: { runtime: "automatic" },
	},
	test: {
		environment: "jsdom",
		setupFiles: ["./vitest.setup.ts"],
		include: [
			"app/**/__tests__/**/*.{ts,tsx,js,jsx}",
			"app/**/*.{test,spec}.{ts,tsx,js,jsx}",
		],
		exclude: [".next/**", "node_modules/**"],
		testTimeout: 10000,
		coverage: {
			provider: "v8",
			include: ["app/**/*.{ts,tsx,js,jsx}"],
			exclude: [
				"app/**/*.d.ts",
				"app/**/index.{ts,tsx,js,jsx}",
				"app/generated/**",
				"app/**/layout.tsx",
				"app/**/page.tsx",
				"app/**/loading.tsx",
				"app/**/error.tsx",
				"app/**/not-found.tsx",
				"app/**/__tests__/**",
				"app/**/*.{test,spec}.{ts,tsx,js,jsx}",
			],
			thresholds: {
				branches: 80,
				functions: 80,
				lines: 80,
				statements: 80,
			},
		},
	},
});
