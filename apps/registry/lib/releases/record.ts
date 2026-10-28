import type { Prisma, PrismaClient } from "@prisma/client";
import { invalidateOnDataChange } from "@/lib/cache-invalidation";
import { compareVersions } from "@/lib/version-compare";
import type { ReleasePayload } from "./contract";

export interface RecordedRelease {
	pluginId: string;
	versionCreated: boolean;
	latestChanged: boolean;
}

async function recordInTransaction(
	db: Prisma.TransactionClient,
	payload: ReleasePayload,
	source: "webhook" | "sync",
): Promise<RecordedRelease> {
	// Serialize even the first release of a plugin. All release writers use this
	// transaction-scoped lock so concurrent older releases cannot regress latest.
	await db.$queryRaw`SELECT pg_advisory_xact_lock(hashtextextended(${payload.plugin}, 0))::text`;
	const metadata = {
		...payload.metadata,
		shortName: payload.metadata.shortName ?? null,
		author: payload.metadata.author ?? null,
		license: payload.metadata.license ?? null,
		homepage: payload.metadata.homepage ?? null,
		repository: payload.metadata.repository ?? null,
		sdkVersion: payload.metadata.sdkVersion ?? null,
		apiVersion: payload.metadata.apiVersion ?? null,
		supports: payload.metadata.supports ?? {},
		priority: payload.metadata.priority ?? 50,
	};
	const plugin = await db.plugin.upsert({
		where: { name: payload.plugin },
		create: { name: payload.plugin, ...metadata },
		update: {},
	});
	const where = {
		pluginId_version: { pluginId: plugin.id, version: payload.version },
	};
	const existing = await db.pluginVersion.findUnique({ where });
	const versionData = {
		tag: payload.tag,
		prerelease: payload.prerelease,
		binaries: payload.binaries,
		checksumsUrl: payload.checksumsUrl ?? null,
		releaseUrl: payload.releaseUrl ?? null,
		commitSha: payload.commitSha ?? null,
		sdkVersion: payload.metadata.sdkVersion ?? null,
		apiVersion: payload.metadata.apiVersion ?? null,
		source,
		publishedAt: new Date(payload.publishedAt),
	};
	await db.pluginVersion.upsert({
		where,
		create: { pluginId: plugin.id, version: payload.version, ...versionData },
		update: versionData,
	});
	const currentLatest = plugin.latestVersion ?? plugin.version;
	const latestChanged =
		!payload.prerelease &&
		(currentLatest === "latest" ||
			compareVersions(payload.version, currentLatest) >= 0);
	if (latestChanged) {
		await db.plugin.update({
			where: { id: plugin.id },
			data: {
				...metadata,
				version: payload.version,
				latestVersion: payload.version,
				binaries: payload.binaries,
			},
		});
	}
	return { pluginId: plugin.id, versionCreated: !existing, latestChanged };
}

/**
 * Record a release atomically, retaining every historical version.
 * With a PrismaClient, invalidation runs after commit. With an existing
 * TransactionClient, the caller MUST invalidateOnDataChange after outer commit;
 * Prisma transactions do not provide an after-commit hook.
 */
export async function recordPluginRelease(
	db: PrismaClient | Prisma.TransactionClient,
	payload: ReleasePayload,
	source: "webhook" | "sync",
): Promise<RecordedRelease> {
	if (!("$transaction" in db)) {
		return recordInTransaction(db, payload, source);
	}
	const result = await db.$transaction((tx) =>
		recordInTransaction(tx, payload, source),
	);
	await invalidateOnDataChange("update", "plugin", result.pluginId);
	return result;
}
