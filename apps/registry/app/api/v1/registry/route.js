import fs from 'node:fs';
import path from 'node:path';
import { NextResponse } from 'next/server';

export async function GET() {
    try {
        // In production, this would read from a database or static file
        // For now, we'll read the generated registry.json
        const registryPath = path.join(process.cwd(), 'public', 'v1', 'registry.json');
        const registry = JSON.parse(fs.readFileSync(registryPath, 'utf8'));

        return NextResponse.json(registry);
    } catch (error) {
        console.error('Failed to load registry:', error);
        return NextResponse.json(
            { error: 'Failed to load plugin registry' },
            { status: 500 }
        );
    }
}

export async function OPTIONS() {
    return new Response(null, { status: 200 });
}
