import { NextResponse } from 'next/server';
import { PrismaClient } from '@prisma/client';

const prisma = new PrismaClient();

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url);
    const type = searchParams.get('type');
    const search = searchParams.get('search');
    const limit = parseInt(searchParams.get('limit') || '50');
    const offset = parseInt(searchParams.get('offset') || '0');

    // Build where clause
    const where: any = {};
    
    if (type) {
      where.type = type;
    }
    
    if (search) {
      where.OR = [
        { name: { contains: search, mode: 'insensitive' } },
        { description: { contains: search, mode: 'insensitive' } },
        { tags: { has: search } },
      ];
    }

    // Fetch plugins with pagination
    const [plugins, total] = await Promise.all([
      prisma.plugin.findMany({
        where,
        orderBy: [
          { priority: 'asc' },
          { name: 'asc' }
        ],
        take: limit,
        skip: offset,
      }),
      prisma.plugin.count({ where })
    ]);

    // Transform to expected format
    const pluginsFormatted = plugins.map(plugin => ({
      name: plugin.name,
      description: plugin.description,
      type: plugin.type,
      priority: plugin.priority,
      status: plugin.status,
      supports: plugin.supports as Record<string, boolean>,
      platforms: plugin.platforms,
      version: '1.1.0',
      author: 'DevEx Team',
      repository: plugin.githubUrl || 'https://github.com/jameswlane/devex',
      githubPath: plugin.githubPath,
      downloadCount: plugin.downloadCount,
      lastDownload: plugin.lastDownload?.toISOString(),
      createdAt: plugin.createdAt.toISOString(),
      updatedAt: plugin.updatedAt.toISOString(),
    }));

    const response = {
      plugins: pluginsFormatted,
      pagination: {
        total,
        count: plugins.length,
        limit,
        offset,
        hasNext: offset + limit < total,
        hasPrevious: offset > 0,
      },
      meta: {
        source: 'database',
        version: '2.0.0',
        timestamp: new Date().toISOString(),
      }
    };

    return NextResponse.json(response, {
      headers: {
        'Cache-Control': 'public, max-age=300, s-maxage=600',
        'X-Total-Count': total.toString(),
        'X-Registry-Source': 'database',
      }
    });

  } catch (error) {
    console.error('Failed to fetch plugins:', error);
    
    return NextResponse.json(
      { 
        error: 'Failed to fetch plugins',
        code: 'DATABASE_ERROR',
        timestamp: new Date().toISOString(),
      },
      { status: 500 }
    );
  } finally {
    await prisma.$disconnect();
  }
}
