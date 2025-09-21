-- CreateTable
CREATE TABLE "public"."applications" (
    "id" TEXT NOT NULL,
    "name" VARCHAR(100) NOT NULL,
    "description" TEXT NOT NULL,
    "category" VARCHAR(50) NOT NULL,
    "official" BOOLEAN NOT NULL DEFAULT false,
    "default" BOOLEAN NOT NULL DEFAULT false,
    "tags" TEXT[],
    "version" VARCHAR(50) NOT NULL DEFAULT 'latest',
    "latestVersion" VARCHAR(50),
    "platforms" JSONB NOT NULL DEFAULT '{}',
    "supportsLinux" BOOLEAN NOT NULL DEFAULT false,
    "supportsMacOS" BOOLEAN NOT NULL DEFAULT false,
    "supportsWindows" BOOLEAN NOT NULL DEFAULT false,
    "desktopEnvironments" TEXT[],
    "githubUrl" VARCHAR(500),
    "githubPath" VARCHAR(500),
    "lastSynced" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "downloadCount" INTEGER NOT NULL DEFAULT 0,
    "lastDownload" TIMESTAMP(3),
    "binaries" JSONB NOT NULL DEFAULT '{}',
    "installMethods" JSONB NOT NULL DEFAULT '{}',
    "dependencies" TEXT[] DEFAULT ARRAY[]::TEXT[],
    "author" VARCHAR(100),
    "license" VARCHAR(50),
    "homepage" VARCHAR(500),
    "repository" VARCHAR(500),
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "applications_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "public"."plugins" (
    "id" TEXT NOT NULL,
    "name" VARCHAR(100) NOT NULL,
    "description" TEXT NOT NULL,
    "type" VARCHAR(50) NOT NULL,
    "priority" INTEGER NOT NULL DEFAULT 50,
    "status" VARCHAR(20) NOT NULL DEFAULT 'active',
    "version" VARCHAR(50) NOT NULL DEFAULT 'latest',
    "latestVersion" VARCHAR(50),
    "supports" JSONB NOT NULL DEFAULT '{}',
    "platforms" TEXT[] DEFAULT ARRAY['linux', 'macos', 'windows']::TEXT[],
    "githubUrl" VARCHAR(500),
    "githubPath" VARCHAR(500),
    "lastSynced" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "downloadCount" INTEGER NOT NULL DEFAULT 0,
    "lastDownload" TIMESTAMP(3),
    "binaries" JSONB NOT NULL DEFAULT '{}',
    "sdkVersion" VARCHAR(50),
    "apiVersion" VARCHAR(50),
    "dependencies" TEXT[] DEFAULT ARRAY[]::TEXT[],
    "conflicts" TEXT[] DEFAULT ARRAY[]::TEXT[],
    "author" VARCHAR(100),
    "license" VARCHAR(50),
    "homepage" VARCHAR(500),
    "repository" VARCHAR(500),
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "plugins_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "public"."configs" (
    "id" TEXT NOT NULL,
    "name" VARCHAR(100) NOT NULL,
    "description" TEXT NOT NULL,
    "category" VARCHAR(50) NOT NULL,
    "type" VARCHAR(20) NOT NULL,
    "version" VARCHAR(50) NOT NULL DEFAULT 'latest',
    "latestVersion" VARCHAR(50),
    "platforms" TEXT[],
    "content" JSONB NOT NULL,
    "schema" JSONB,
    "githubUrl" VARCHAR(500),
    "githubPath" VARCHAR(500),
    "lastSynced" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "downloadCount" INTEGER NOT NULL DEFAULT 0,
    "lastDownload" TIMESTAMP(3),
    "binaries" JSONB NOT NULL DEFAULT '{}',
    "dependencies" TEXT[] DEFAULT ARRAY[]::TEXT[],
    "conflicts" TEXT[] DEFAULT ARRAY[]::TEXT[],
    "variables" JSONB NOT NULL DEFAULT '{}',
    "author" VARCHAR(100),
    "license" VARCHAR(50),
    "homepage" VARCHAR(500),
    "repository" VARCHAR(500),
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "configs_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "public"."stacks" (
    "id" TEXT NOT NULL,
    "name" VARCHAR(100) NOT NULL,
    "description" TEXT NOT NULL,
    "category" VARCHAR(50) NOT NULL,
    "version" VARCHAR(50) NOT NULL DEFAULT 'latest',
    "latestVersion" VARCHAR(50),
    "applications" TEXT[],
    "configs" TEXT[],
    "plugins" TEXT[],
    "platforms" TEXT[],
    "desktopEnvironments" TEXT[],
    "prerequisites" JSONB NOT NULL DEFAULT '[]',
    "githubUrl" VARCHAR(500),
    "githubPath" VARCHAR(500),
    "lastSynced" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "downloadCount" INTEGER NOT NULL DEFAULT 0,
    "lastDownload" TIMESTAMP(3),
    "binaries" JSONB NOT NULL DEFAULT '{}',
    "dependencies" TEXT[] DEFAULT ARRAY[]::TEXT[],
    "conflicts" TEXT[] DEFAULT ARRAY[]::TEXT[],
    "variables" JSONB NOT NULL DEFAULT '{}',
    "author" VARCHAR(100),
    "license" VARCHAR(50),
    "homepage" VARCHAR(500),
    "repository" VARCHAR(500),
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "stacks_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "public"."registry_stats" (
    "id" TEXT NOT NULL,
    "date" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "totalApplications" INTEGER NOT NULL DEFAULT 0,
    "totalPlugins" INTEGER NOT NULL DEFAULT 0,
    "totalConfigs" INTEGER NOT NULL DEFAULT 0,
    "totalStacks" INTEGER NOT NULL DEFAULT 0,
    "linuxSupported" INTEGER NOT NULL DEFAULT 0,
    "macosSupported" INTEGER NOT NULL DEFAULT 0,
    "windowsSupported" INTEGER NOT NULL DEFAULT 0,
    "totalDownloads" INTEGER NOT NULL DEFAULT 0,
    "dailyDownloads" INTEGER NOT NULL DEFAULT 0,

    CONSTRAINT "registry_stats_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "public"."sync_logs" (
    "id" TEXT NOT NULL,
    "type" VARCHAR(20) NOT NULL,
    "name" VARCHAR(100) NOT NULL,
    "action" VARCHAR(20) NOT NULL,
    "githubUrl" VARCHAR(500),
    "success" BOOLEAN NOT NULL,
    "error" TEXT,
    "changes" JSONB,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "sync_logs_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE UNIQUE INDEX "applications_name_key" ON "public"."applications"("name");

-- CreateIndex
CREATE INDEX "applications_category_idx" ON "public"."applications"("category");

-- CreateIndex
CREATE INDEX "applications_official_idx" ON "public"."applications"("official");

-- CreateIndex
CREATE INDEX "applications_default_idx" ON "public"."applications"("default");

-- CreateIndex
CREATE INDEX "applications_name_idx" ON "public"."applications"("name");

-- CreateIndex
CREATE INDEX "applications_supportsLinux_idx" ON "public"."applications"("supportsLinux");

-- CreateIndex
CREATE INDEX "applications_supportsMacOS_idx" ON "public"."applications"("supportsMacOS");

-- CreateIndex
CREATE INDEX "applications_supportsWindows_idx" ON "public"."applications"("supportsWindows");

-- CreateIndex
CREATE INDEX "applications_platforms_idx" ON "public"."applications" USING GIN ("platforms");

-- CreateIndex
CREATE INDEX "idx_applications_official_default_name" ON "public"."applications"("official" DESC, "default" DESC, "name" ASC);

-- CreateIndex
CREATE INDEX "idx_applications_category_official" ON "public"."applications"("category", "official" DESC);

-- CreateIndex
CREATE INDEX "idx_applications_category_linux" ON "public"."applications"("category", "supportsLinux");

-- CreateIndex
CREATE INDEX "idx_applications_category_macos" ON "public"."applications"("category", "supportsMacOS");

-- CreateIndex
CREATE INDEX "idx_applications_category_windows" ON "public"."applications"("category", "supportsWindows");

-- CreateIndex
CREATE UNIQUE INDEX "plugins_name_key" ON "public"."plugins"("name");

-- CreateIndex
CREATE INDEX "plugins_type_idx" ON "public"."plugins"("type");

-- CreateIndex
CREATE INDEX "plugins_priority_idx" ON "public"."plugins"("priority");

-- CreateIndex
CREATE INDEX "plugins_status_idx" ON "public"."plugins"("status");

-- CreateIndex
CREATE INDEX "plugins_name_idx" ON "public"."plugins"("name");

-- CreateIndex
CREATE INDEX "idx_plugins_type_status" ON "public"."plugins"("type", "status");

-- CreateIndex
CREATE INDEX "idx_plugins_status_priority_name" ON "public"."plugins"("status", "priority" DESC, "name" ASC);

-- CreateIndex
CREATE INDEX "idx_plugins_type_priority" ON "public"."plugins"("type", "priority" DESC);

-- CreateIndex
CREATE INDEX "idx_plugins_downloads_recent" ON "public"."plugins"("downloadCount" DESC, "lastDownload" DESC);

-- CreateIndex
CREATE UNIQUE INDEX "configs_name_key" ON "public"."configs"("name");

-- CreateIndex
CREATE INDEX "configs_category_idx" ON "public"."configs"("category");

-- CreateIndex
CREATE INDEX "configs_type_idx" ON "public"."configs"("type");

-- CreateIndex
CREATE INDEX "configs_name_idx" ON "public"."configs"("name");

-- CreateIndex
CREATE INDEX "idx_configs_category_type" ON "public"."configs"("category", "type");

-- CreateIndex
CREATE INDEX "idx_configs_category_downloads" ON "public"."configs"("category", "downloadCount" DESC);

-- CreateIndex
CREATE INDEX "idx_configs_downloads_recent" ON "public"."configs"("downloadCount" DESC, "lastDownload" DESC);

-- CreateIndex
CREATE UNIQUE INDEX "stacks_name_key" ON "public"."stacks"("name");

-- CreateIndex
CREATE INDEX "stacks_category_idx" ON "public"."stacks"("category");

-- CreateIndex
CREATE INDEX "stacks_name_idx" ON "public"."stacks"("name");

-- CreateIndex
CREATE INDEX "idx_stacks_category_downloads" ON "public"."stacks"("category", "downloadCount" DESC);

-- CreateIndex
CREATE INDEX "idx_stacks_downloads_recent" ON "public"."stacks"("downloadCount" DESC, "lastDownload" DESC);

-- CreateIndex
CREATE UNIQUE INDEX "registry_stats_date_key" ON "public"."registry_stats"("date");

-- CreateIndex
CREATE INDEX "sync_logs_type_idx" ON "public"."sync_logs"("type");

-- CreateIndex
CREATE INDEX "sync_logs_createdAt_idx" ON "public"."sync_logs"("createdAt");

-- CreateIndex
CREATE INDEX "sync_logs_success_idx" ON "public"."sync_logs"("success");

-- CreateIndex
CREATE INDEX "idx_sync_logs_type_created" ON "public"."sync_logs"("type", "createdAt" DESC);

-- CreateIndex
CREATE INDEX "idx_sync_logs_success_created" ON "public"."sync_logs"("success", "createdAt" DESC);
