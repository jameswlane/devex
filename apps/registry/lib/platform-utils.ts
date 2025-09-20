/**
 * Platform utilities for maintaining synchronized platform support data
 * Ensures boolean columns stay in sync with JSON platform data
 */

export interface PlatformData {
  linux?: {
    supported?: boolean;
    installMethod?: string;
    installCommand?: string;
    officialSupport?: boolean;
    alternatives?: string[];
  };
  macos?: {
    supported?: boolean;
    installMethod?: string;
    installCommand?: string;
    officialSupport?: boolean;
    alternatives?: string[];
  };
  windows?: {
    supported?: boolean;
    installMethod?: string;
    installCommand?: string;
    officialSupport?: boolean;
    alternatives?: string[];
  };
}

export interface PlatformSupport {
  supportsLinux: boolean;
  supportsMacOS: boolean;
  supportsWindows: boolean;
}

/**
 * Extract platform support flags from JSON platform data
 * This function analyzes the platform JSON structure and determines
 * which platforms are actually supported
 */
export function extractPlatformSupport(platforms: any): PlatformSupport {
  const platformData = platforms as PlatformData;
  
  return {
    supportsLinux: !!(
      platformData?.linux?.supported === true ||
      (platformData?.linux && platformData.linux.supported !== false && 
       (platformData.linux.installMethod || platformData.linux.installCommand))
    ),
    supportsMacOS: !!(
      platformData?.macos?.supported === true ||
      (platformData?.macos && platformData.macos.supported !== false && 
       (platformData.macos.installMethod || platformData.macos.installCommand))
    ),
    supportsWindows: !!(
      platformData?.windows?.supported === true ||
      (platformData?.windows && platformData.windows.supported !== false && 
       (platformData.windows.installMethod || platformData.windows.installCommand))
    )
  };
}

/**
 * Create application data with synchronized platform support
 * Use this when creating new applications to ensure consistency
 */
export function createApplicationDataWithPlatforms(
  applicationData: any,
  platforms: PlatformData
): any {
  const platformSupport = extractPlatformSupport(platforms);
  
  return {
    ...applicationData,
    platforms,
    ...platformSupport
  };
}

/**
 * Validate platform data structure
 * Ensures the platform JSON follows the expected schema
 */
export function validatePlatformData(platforms: any): boolean {
  if (!platforms || typeof platforms !== 'object') {
    return false;
  }
  
  const validPlatforms = ['linux', 'macos', 'windows'];
  
  for (const [platform, config] of Object.entries(platforms)) {
    if (!validPlatforms.includes(platform)) {
      continue; // Allow unknown platforms for future extensibility
    }
    
    if (config && typeof config === 'object') {
      const platformConfig = config as any;
      
      // Validate boolean fields
      if (platformConfig.supported !== undefined && typeof platformConfig.supported !== 'boolean') {
        return false;
      }
      
      if (platformConfig.officialSupport !== undefined && typeof platformConfig.officialSupport !== 'boolean') {
        return false;
      }
      
      // Validate string fields
      if (platformConfig.installMethod !== undefined && typeof platformConfig.installMethod !== 'string') {
        return false;
      }
      
      if (platformConfig.installCommand !== undefined && typeof platformConfig.installCommand !== 'string') {
        return false;
      }
      
      // Validate array fields
      if (platformConfig.alternatives !== undefined && !Array.isArray(platformConfig.alternatives)) {
        return false;
      }
    }
  }
  
  return true;
}

/**
 * Generate platform statistics for analytics
 * Used by the stats API to provide platform distribution data
 */
export function calculatePlatformStats(applications: Array<{ 
  supportsLinux: boolean;
  supportsMacOS: boolean;
  supportsWindows: boolean;
}>): { linux: number; macos: number; windows: number } {
  return applications.reduce(
    (stats, app) => ({
      linux: stats.linux + (app.supportsLinux ? 1 : 0),
      macos: stats.macos + (app.supportsMacOS ? 1 : 0),
      windows: stats.windows + (app.supportsWindows ? 1 : 0)
    }),
    { linux: 0, macos: 0, windows: 0 }
  );
}

/**
 * Migration utility to update existing records
 * Use this to backfill platform support columns from JSON data
 */
export async function syncPlatformColumns(prisma: any): Promise<{ updated: number; errors: number }> {
  let updated = 0;
  let errors = 0;
  
  try {
    // Get all applications with their platform data
    const applications = await prisma.application.findMany({
      select: {
        id: true,
        platforms: true,
        supportsLinux: true,
        supportsMacOS: true,
        supportsWindows: true
      }
    });
    
    // Update each application with correct platform support flags
    for (const app of applications) {
      try {
        const platformSupport = extractPlatformSupport(app.platforms);
        
        // Only update if there's a mismatch
        if (
          app.supportsLinux !== platformSupport.supportsLinux ||
          app.supportsMacOS !== platformSupport.supportsMacOS ||
          app.supportsWindows !== platformSupport.supportsWindows
        ) {
          await prisma.application.update({
            where: { id: app.id },
            data: platformSupport
          });
          updated++;
        }
      } catch (error) {
        console.error(`Error updating application ${app.id}:`, error);
        errors++;
      }
    }
    
    return { updated, errors };
  } catch (error) {
    console.error('Error in syncPlatformColumns:', error);
    throw error;
  }
}