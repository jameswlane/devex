-- CreateTable
CREATE TABLE "plugin_versions" (
    "id" TEXT NOT NULL,
    "pluginId" TEXT NOT NULL,
    "version" VARCHAR(50) NOT NULL,
    "tag" VARCHAR(200) NOT NULL,
    "prerelease" BOOLEAN NOT NULL DEFAULT false,
    "binaries" JSONB NOT NULL DEFAULT '{}',
    "checksumsUrl" VARCHAR(500),
    "releaseUrl" VARCHAR(500),
    "commitSha" VARCHAR(64),
    "sdkVersion" VARCHAR(50),
    "apiVersion" VARCHAR(50),
    "source" VARCHAR(20) NOT NULL DEFAULT 'webhook',
    "publishedAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "plugin_versions_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE INDEX "plugin_versions_pluginId_publishedAt_idx" ON "plugin_versions"("pluginId", "publishedAt" DESC);

-- CreateIndex
CREATE UNIQUE INDEX "plugin_versions_pluginId_version_key" ON "plugin_versions"("pluginId", "version");

-- AddForeignKey
ALTER TABLE "plugin_versions" ADD CONSTRAINT "plugin_versions_pluginId_fkey" FOREIGN KEY ("pluginId") REFERENCES "plugins"("id") ON DELETE CASCADE ON UPDATE CASCADE;

