/**
 * @jest-environment jsdom
 */
import {
	afterEach,
	beforeEach,
	describe,
	expect,
	it,
	jest,
} from "@jest/globals";
import { act, render } from "@testing-library/react";
import type { ComponentType, ReactElement } from "react";
import { ApplicationsClient } from "@/app/applications/applications-client";
import { PluginsClient } from "@/app/plugins/plugins-client";
import { SEARCH_DEBOUNCE_MS } from "@/lib/constants";

const push = jest.fn<(url: string) => void>();
let currentQuery = "";

jest.mock("next/navigation", () => ({
	useRouter: () => ({ push }),
	useSearchParams: () => new URLSearchParams(currentQuery),
}));

interface CapturedPanel {
	searchQuery: string;
	onSearchChange: (value: string) => void;
	filters: { id: string; onChange: (value: string) => void }[];
	onClearFilters: () => void;
}

const panel: { current?: CapturedPanel } = {};

jest.mock("@/components/filter-panel", () => ({
	FilterPanel: (props: CapturedPanel) => {
		panel.current = props;
		return null;
	},
}));
jest.mock("@/components/pagination", () => ({ Pagination: () => null }));
jest.mock("@/components/error-boundary", () => ({
	APIErrorBoundary: ({ children }: { children: unknown }) => children,
}));
jest.mock("@/components/application-card", () => ({
	ApplicationCard: () => null,
}));
jest.mock("@/components/plugin-card", () => ({ PluginCard: () => null }));

type Params = Record<string, string>;

interface ClientCase {
	name: string;
	Client: ComponentType<{ initialData: never; initialParams: never }>;
	baseParams: Params;
	filterId: string;
	filterValue: string;
}

const cases: ClientCase[] = [
	{
		name: "ApplicationsClient",
		Client: ApplicationsClient as ClientCase["Client"],
		baseParams: { page: "1", limit: "20", category: "", platform: "" },
		filterId: "category",
		filterValue: "development",
	},
	{
		name: "PluginsClient",
		Client: PluginsClient as ClientCase["Client"],
		baseParams: { page: "1", limit: "20", type: "", status: "" },
		filterId: "type",
		filterValue: "package-manager",
	},
];

const initialData = { items: [], pagination: {} } as never;

function searchOf(url: string): string | null {
	return new URLSearchParams(url.split("?")[1] ?? "").get("search");
}

describe.each(cases)("$name search URL sync", (testCase) => {
	const { Client, baseParams } = testCase;

	// Simulates a completed navigation to ?search=<search>
	function navigate(rerender: (ui: ReactElement) => void, search: string) {
		currentQuery = search ? `search=${search}` : "";
		rerender(
			<Client
				initialData={initialData}
				initialParams={{ ...baseParams, search } as never}
			/>,
		);
	}

	function renderClient(search = "") {
		currentQuery = search ? `search=${search}` : "";
		return render(
			<Client
				initialData={initialData}
				initialParams={{ ...baseParams, search } as never}
			/>,
		);
	}

	function type(value: string) {
		act(() => panel.current?.onSearchChange(value));
		act(() => {
			jest.advanceTimersByTime(SEARCH_DEBOUNCE_MS);
		});
	}

	beforeEach(() => {
		jest.useFakeTimers();
		push.mockClear();
		panel.current = undefined;
	});

	afterEach(() => {
		jest.useRealTimers();
	});

	it("does not push on first render", () => {
		renderClient("abc");
		act(() => {
			jest.advanceTimersByTime(SEARCH_DEBOUNCE_MS);
		});
		expect(push).not.toHaveBeenCalled();
	});

	it("pushes once when the debounced search changes", () => {
		const { rerender } = renderClient();
		type("abc");
		expect(push).toHaveBeenCalledTimes(1);
		expect(searchOf(push.mock.calls[0][0])).toBe("abc");

		navigate(rerender, "abc");
		act(() => {
			jest.advanceTimersByTime(SEARCH_DEBOUNCE_MS);
		});
		expect(push).toHaveBeenCalledTimes(1);
	});

	it("restores the search on back/forward without pushing", () => {
		const { rerender } = renderClient();
		type("abc");
		navigate(rerender, "abc");

		navigate(rerender, "");
		act(() => {
			jest.advanceTimersByTime(SEARCH_DEBOUNCE_MS);
		});
		expect(panel.current?.searchQuery).toBe("");

		navigate(rerender, "abc");
		act(() => {
			jest.advanceTimersByTime(SEARCH_DEBOUNCE_MS);
		});
		expect(panel.current?.searchQuery).toBe("abc");
		expect(push).toHaveBeenCalledTimes(1);
	});

	it("still pushes new input after history navigation", () => {
		const { rerender } = renderClient();
		type("abc");
		navigate(rerender, "abc");
		navigate(rerender, "");

		type("xyz");
		expect(push).toHaveBeenCalledTimes(2);
		expect(searchOf(push.mock.calls[1][0])).toBe("xyz");
	});

	it("keeps a pending search when a filter changes before navigation completes", () => {
		renderClient();
		type("abc");

		// searchParams still reflects the old URL: the search push hasn't landed
		const filter = panel.current?.filters.find(
			(f) => f.id === testCase.filterId,
		);
		act(() => filter?.onChange(testCase.filterValue));

		const filterUrl = push.mock.calls[1][0];
		expect(searchOf(filterUrl)).toBe("abc");
		expect(filterUrl).toContain(`${testCase.filterId}=${testCase.filterValue}`);
	});

	it("drops the search from filter URLs after clearing filters", () => {
		const { rerender } = renderClient();
		type("abc");
		navigate(rerender, "abc");

		act(() => panel.current?.onClearFilters());
		const filter = panel.current?.filters.find(
			(f) => f.id === testCase.filterId,
		);
		act(() => filter?.onChange(testCase.filterValue));

		expect(searchOf(push.mock.calls[push.mock.calls.length - 1][0])).toBeNull();
	});

	it("pushes a search retyped within the debounce interval after clearing filters", () => {
		const { rerender } = renderClient();
		type("abc");
		navigate(rerender, "abc");

		act(() => panel.current?.onClearFilters());
		navigate(rerender, "");
		type("abc");

		const lastUrl = push.mock.calls[push.mock.calls.length - 1][0];
		expect(searchOf(lastUrl)).toBe("abc");
	});

	it("drops a pending search when history navigation happens first", () => {
		const { rerender } = renderClient();
		type("abc");
		navigate(rerender, "abc");

		act(() => panel.current?.onSearchChange("abcd"));
		navigate(rerender, "");
		act(() => {
			jest.advanceTimersByTime(SEARCH_DEBOUNCE_MS);
		});

		expect(push).toHaveBeenCalledTimes(1);
		expect(panel.current?.searchQuery).toBe("");
	});
});
