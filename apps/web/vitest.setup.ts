import "@testing-library/jest-dom/vitest";
import { ReadableStream } from "node:stream/web";
import { TextDecoder, TextEncoder } from "node:util";
import { MessagePort } from "node:worker_threads";
import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";

// Polyfill Node.js globals missing from the jsdom environment
if (typeof global.TextEncoder === "undefined") {
	global.TextEncoder = TextEncoder;
}
if (typeof global.TextDecoder === "undefined") {
	global.TextDecoder = TextDecoder;
}
if (typeof global.ReadableStream === "undefined") {
	global.ReadableStream =
		ReadableStream as unknown as typeof global.ReadableStream;
}
if (typeof global.MessagePort === "undefined") {
	global.MessagePort = MessagePort;
}

// Unmount React trees between tests (Testing Library only does this on its own
// when test globals are enabled)
afterEach(() => {
	cleanup();
});

// Mock Next.js router
vi.mock("next/router", () => ({
	useRouter() {
		return {
			route: "/",
			pathname: "/",
			query: {},
			asPath: "/",
			push: vi.fn(),
			pop: vi.fn(),
			reload: vi.fn(),
			back: vi.fn(),
			prefetch: vi.fn().mockResolvedValue(undefined),
			beforePopState: vi.fn(),
			events: {
				on: vi.fn(),
				off: vi.fn(),
				emit: vi.fn(),
			},
		};
	},
}));

// Mock Next.js navigation
vi.mock("next/navigation", () => ({
	useRouter() {
		return {
			push: vi.fn(),
			replace: vi.fn(),
			refresh: vi.fn(),
			back: vi.fn(),
			forward: vi.fn(),
			prefetch: vi.fn(),
		};
	},
	usePathname() {
		return "/";
	},
	useSearchParams() {
		return new URLSearchParams();
	},
}));

// Mock fetch globally
global.fetch = vi.fn();

// Mock window.matchMedia (only in jsdom environment)
if (typeof window !== "undefined") {
	Object.defineProperty(window, "matchMedia", {
		writable: true,
		value: vi.fn().mockImplementation((query) => ({
			matches: false,
			media: query,
			onchange: null,
			addListener: vi.fn(), // deprecated
			removeListener: vi.fn(), // deprecated
			addEventListener: vi.fn(),
			removeEventListener: vi.fn(),
			dispatchEvent: vi.fn(),
		})),
	});
}

// Mock IntersectionObserver
global.IntersectionObserver = vi.fn(
	class {
		observe = vi.fn();
		unobserve = vi.fn();
		disconnect = vi.fn();
	},
) as unknown as typeof IntersectionObserver;

// Mock ResizeObserver
global.ResizeObserver = vi.fn(
	class {
		observe = vi.fn();
		unobserve = vi.fn();
		disconnect = vi.fn();
	},
) as unknown as typeof ResizeObserver;

// Suppress React 18 warnings in tests
const originalError = console.error;
console.error = (...args) => {
	if (
		args[0]?.includes?.("Warning: ReactDOM.render is no longer supported") ||
		args[0]?.includes?.("Warning: React.createFactory() is deprecated")
	) {
		return;
	}
	originalError(...args);
};
