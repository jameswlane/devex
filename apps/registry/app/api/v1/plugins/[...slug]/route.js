import fs from 'node:fs';
import path from 'node:path';
import { NextResponse } from 'next/server';

export async function GET(request, { params }) {
    const { slug } = await params;

    try {
        const registryPath = path.join(process.cwd(), 'public', 'v1', 'registry.json');
        const registry = JSON.parse(fs.readFileSync(registryPath, 'utf8'));

        if (slug.length === 1) {
            // Single plugin request: /api/v1/plugins/package-manager-apt
            const pluginName = slug[0];
            const plugin = registry.plugins[pluginName];

            if (!plugin) {
                return NextResponse.json(
                    { error: 'Plugin not found' },
                    { status: 404 }
                );
            }

            return NextResponse.json(plugin);
        } else if (slug.length === 0) {
            // All plugins request: /api/v1/plugins
            return NextResponse.json(registry.plugins);
        } else {
            return NextResponse.json(
                { error: 'Invalid request path' },
                { status: 400 }
            );
        }
    } catch (error) {
        console.error('Failed to load plugins:', error);
        return NextResponse.json(
            { error: 'Failed to load plugins' },
            { status: 500 }
        );
    }
}

export async function OPTIONS() {
    return new Response(null, { status: 200 });
}
