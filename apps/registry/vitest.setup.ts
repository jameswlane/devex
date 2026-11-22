import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterAll, afterEach, beforeEach, vi } from "vitest";

// Mock environment variables for testing
Object.assign(process.env, {
	NODE_ENV: "test",
	DATABASE_URL: "file:./test.db",
	NEXTAUTH_SECRET: "test-secret",
	NEXTAUTH_URL: "http://localhost:3000",
	// Empty, not deleted: dotenv never overrides a set variable, so a local
	// .env cannot point tests at a live Redis
	KV_REST_API_URL: "",
	KV_REST_API_TOKEN: "",
	UPSTASH_REDIS_REST_URL: "",
	UPSTASH_REDIS_REST_TOKEN: "",
	REDIS_URL: "",
});

// Suppress console output during tests unless it's an error
const originalConsole = console;
global.console = {
	...originalConsole,
	log: vi.fn(),
	debug: vi.fn(),
	info: vi.fn(),
	warn: vi.fn(),
	error: originalConsole.error, // Keep errors visible
};

// Mock Prisma client for tests
vi.mock("./lib/prisma", () => ({
	prisma: {
		$transaction: vi.fn(),
		plugin: {
			findMany: vi.fn(),
			count: vi.fn(),
		},
		application: {
			findMany: vi.fn(),
			count: vi.fn(),
		},
		config: {
			findMany: vi.fn(),
			count: vi.fn(),
		},
		stack: {
			findMany: vi.fn(),
			count: vi.fn(),
		},
		registryStats: {
			findFirst: vi.fn(),
		},
	},
}));

// Reset all mocks before each test
beforeEach(() => {
	vi.clearAllMocks();
});

// Unmount React trees between tests (Testing Library only does this on its own
// when test globals are enabled)
afterEach(() => {
	cleanup();
});

// Clean up any open handles after all tests
afterAll(() => {
	// Clear all timers
	vi.clearAllTimers();
	// Force exit if needed
	if (global.gc) {
		global.gc();
	}
});
