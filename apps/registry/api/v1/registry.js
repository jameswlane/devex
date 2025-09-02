import fs from 'node:fs';
import path from 'node:path';

export default function handler(req, res) {
    if (req.method === 'OPTIONS') {
        res.status(200).end();
        return;
    }

    if (req.method !== 'GET') {
        res.status(405).json({ error: 'Method not allowed' });
        return;
    }

    try {
        // In production, this would read from a database or static file
        // For now, we'll read the generated registry.json
        const registryPath = path.join(process.cwd(), 'public', 'v1', 'registry.json');
        const registry = JSON.parse(fs.readFileSync(registryPath, 'utf8'));

        res.status(200).json(registry);
    } catch (error) {
        console.error('Failed to load registry:', error);
        res.status(500).json({ error: 'Failed to load plugin registry' });
    }
}
