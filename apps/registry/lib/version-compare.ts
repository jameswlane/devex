/**
 * Compare two semantic versions ("1.2.3", "1.2.3-rc.1"). Returns <0, 0 or >0.
 * Pre-releases sort before their release, per semver precedence rules.
 */
export function compareVersions(a: string, b: string): number {
	const parse = (v: string) => {
		const [core, pre] = v.replace(/^v/, "").split("+")[0].split(/-(.*)/);
		return {
			nums: core.split(".").map((n) => Number.parseInt(n, 10) || 0),
			pre: pre ? pre.split(".") : [],
		};
	};
	const x = parse(a);
	const y = parse(b);

	for (let i = 0; i < 3; i++) {
		const diff = (x.nums[i] ?? 0) - (y.nums[i] ?? 0);
		if (diff !== 0) return diff;
	}
	if (x.pre.length === 0 || y.pre.length === 0)
		return y.pre.length - x.pre.length;

	for (let i = 0; i < Math.max(x.pre.length, y.pre.length); i++) {
		const p = x.pre[i];
		const q = y.pre[i];
		if (p === undefined) return -1;
		if (q === undefined) return 1;
		const pn = /^\d+$/.test(p);
		const qn = /^\d+$/.test(q);
		if (pn && qn) {
			const diff = Number(p) - Number(q);
			if (diff !== 0) return diff;
		} else if (pn !== qn) {
			return pn ? -1 : 1;
		} else if (p !== q) {
			return p < q ? -1 : 1;
		}
	}
	return 0;
}
