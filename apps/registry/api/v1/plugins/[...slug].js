import fs from 'node:fs';
import path from 'node:path';

export default function handler(req, res) {
    const { slug } = req.query;

    if (req.method === 'OPTIONS') {
        res.status(200).end();
        return;
    }

    if (req.method !== 'GET') {
        res.status(405).json({ error: 'Method not allowed' });
        return;
    }

    try {
        const registryPath = path.join(process.cwd(), 'public', 'v1', 'registry.json');
        const registry = JSON.parse(fs.readFileSync(registryPath, 'utf8'));

        if (slug.length === 1) {
            // Single plugin request: /api/v1/plugins/package-manager-apt
            const pluginName = slug[0];
            const plugin = registry.plugins[pluginName];

            if (!plugin) {
                res.status(404).json({ error: 'Plugin not found' });
                return;
            }

            res.status(200).json(plugin);
        } else if (slug.length === 0) {
            // All plugins request: /api/v1/plugins
            res.status(200).json(registry.plugins);
        } else {
            res.status(400).json({ error: 'Invalid request path' });
        }
    } catch (error) {
        console.error('Failed to load plugins:', error);
        res.status(500).json({ error: 'Failed to load plugins' });
    }
}
