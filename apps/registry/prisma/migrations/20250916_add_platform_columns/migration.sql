-- AddPlatformColumns
-- Migration to add optimized boolean platform support columns

-- Add new boolean columns for platform support
ALTER TABLE "applications" ADD COLUMN "supportsLinux" BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE "applications" ADD COLUMN "supportsMacOS" BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE "applications" ADD COLUMN "supportsWindows" BOOLEAN NOT NULL DEFAULT false;

-- Create indexes for efficient platform filtering
CREATE INDEX "applications_supportsLinux_idx" ON "applications"("supportsLinux");
CREATE INDEX "applications_supportsMacOS_idx" ON "applications"("supportsMacOS");
CREATE INDEX "applications_supportsWindows_idx" ON "applications"("supportsWindows");

-- Create composite indexes for category + platform queries
CREATE INDEX "idx_applications_category_linux" ON "applications"("category", "supportsLinux");
CREATE INDEX "idx_applications_category_macos" ON "applications"("category", "supportsMacOS");
CREATE INDEX "idx_applications_category_windows" ON "applications"("category", "supportsWindows");

-- Populate the new columns from existing JSON data
-- This handles the current JSON structure where platforms is an object like:
-- { "linux": { "supported": true, ... }, "macos": { "supported": false, ... } }

UPDATE "applications" 
SET "supportsLinux" = true 
WHERE (platforms->'linux'->>'supported')::boolean = true 
   OR (platforms->>'linux') IS NOT NULL AND platforms->>'linux' != 'null'
   OR platforms ? 'linux';

UPDATE "applications" 
SET "supportsMacOS" = true 
WHERE (platforms->'macos'->>'supported')::boolean = true 
   OR (platforms->>'macos') IS NOT NULL AND platforms->>'macos' != 'null'
   OR platforms ? 'macos';

UPDATE "applications" 
SET "supportsWindows" = true 
WHERE (platforms->'windows'->>'supported')::boolean = true 
   OR (platforms->>'windows') IS NOT NULL AND platforms->>'windows' != 'null'
   OR platforms ? 'windows';

-- Add comment explaining the optimization
COMMENT ON COLUMN "applications"."supportsLinux" IS 'Optimized boolean column for Linux platform support queries';
COMMENT ON COLUMN "applications"."supportsMacOS" IS 'Optimized boolean column for macOS platform support queries';
COMMENT ON COLUMN "applications"."supportsWindows" IS 'Optimized boolean column for Windows platform support queries';