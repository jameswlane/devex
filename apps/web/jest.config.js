/** @type {import('jest').Config} */
const config = {
	testEnvironment: "jsdom",
	setupFilesAfterEnv: ["<rootDir>/jest.setup.js"],
	testPathIgnorePatterns: ["<rootDir>/.next/", "<rootDir>/node_modules/"],
	moduleNameMapper: {
		"^@/(.*)$": "<rootDir>/app/$1",
		"^@/components/(.*)$": "<rootDir>/app/components/$1",
		"^@/utils/(.*)$": "<rootDir>/app/utils/$1",
		"^@/generated/(.*)$": "<rootDir>/app/generated/$1",
	},
	transform: {
		"^.+\\.(ts|tsx)$": [
			"@swc/jest",
			{
				jsc: {
					parser: {
						syntax: "typescript",
						tsx: true,
					},
					transform: {
						react: {
							runtime: "automatic",
						},
					},
				},
			},
		],
		"^.+\\.(js|jsx)$": [
			"@swc/jest",
			{
				jsc: {
					parser: {
						syntax: "ecmascript",
						jsx: true,
					},
					transform: {
						react: {
							runtime: "automatic",
						},
					},
				},
			},
		],
	},
	moduleFileExtensions: ["ts", "tsx", "js", "jsx", "json"],
	collectCoverageFrom: [
		"app/**/*.{ts,tsx,js,jsx}",
		"!app/**/*.d.ts",
		"!app/**/index.{ts,tsx,js,jsx}",
		"!app/generated/**/*",
		"!app/**/layout.tsx",
		"!app/**/page.tsx",
		"!app/**/loading.tsx",
		"!app/**/error.tsx",
		"!app/**/not-found.tsx",
	],
	coverageThreshold: {
		global: {
			branches: 80,
			functions: 80,
			lines: 80,
			statements: 80,
		},
	},
	testMatch: [
		"<rootDir>/app/**/__tests__/**/*.{ts,tsx,js,jsx}",
		"<rootDir>/app/**/*.{test,spec}.{ts,tsx,js,jsx}",
	],
	moduleDirectories: ["node_modules", "<rootDir>"],
	extensionsToTreatAsEsm: [".ts", ".tsx"],
	testTimeout: 10000,
};

module.exports = config;
