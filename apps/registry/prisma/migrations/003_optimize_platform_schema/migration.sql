-- Optimize application platform support schema
-- Replace complex PlatformInfo relationships with embedded JSON for better performance

-- Step 1: Add new platforms JSONB column to applications table
ALTER TABLE applications ADD COLUMN platforms JSONB DEFAULT '{}';

-- Step 2: Migrate existing platform support data to new JSON structure
-- This migration preserves all existing data by converting the relational data to JSON
UPDATE applications 
SET platforms = (
    SELECT jsonb_build_object(
        'linux', CASE 
            WHEN linux_platform.id IS NOT NULL THEN jsonb_build_object(
                'installMethod', linux_platform.install_method,
                'installCommand', linux_platform.install_command,
                'officialSupport', linux_platform.official_support,
                'alternatives', linux_platform.alternatives
            )
            ELSE null
        END,
        'macos', CASE 
            WHEN macos_platform.id IS NOT NULL THEN jsonb_build_object(
                'installMethod', macos_platform.install_method,
                'installCommand', macos_platform.install_command,
                'officialSupport', macos_platform.official_support,
                'alternatives', macos_platform.alternatives
            )
            ELSE null
        END,
        'windows', CASE 
            WHEN windows_platform.id IS NOT NULL THEN jsonb_build_object(
                'installMethod', windows_platform.install_method,
                'installCommand', windows_platform.install_command,
                'officialSupport', windows_platform.official_support,
                'alternatives', windows_platform.alternatives
            )
            ELSE null
        END
    )
    FROM applications app
    LEFT JOIN platform_info linux_platform ON app.linux_support_id = linux_platform.id
    LEFT JOIN platform_info macos_platform ON app.macos_support_id = macos_platform.id  
    LEFT JOIN platform_info windows_platform ON app.windows_support_id = windows_platform.id
    WHERE app.id = applications.id
);

-- Step 3: Create optimized GIN index for JSON platform queries
CREATE INDEX idx_applications_platforms ON applications USING GIN (platforms);

-- Step 4: Remove old foreign key constraints and columns
ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_linux_support_id_fkey;
ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_macos_support_id_fkey;
ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_windows_support_id_fkey;

ALTER TABLE applications DROP COLUMN IF EXISTS linux_support_id;
ALTER TABLE applications DROP COLUMN IF EXISTS macos_support_id;
ALTER TABLE applications DROP COLUMN IF EXISTS windows_support_id;

-- Step 5: Drop the PlatformInfo table as it's no longer needed
DROP TABLE IF EXISTS platform_info;

-- Step 6: Update registry stats if needed to maintain referential integrity
-- This ensures no orphaned references remain in the system