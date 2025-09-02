#!/usr/bin/env node

const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

/**
 * Analyzes git commits to determine semantic version bump
 * Based on Conventional Commits specification
 */
class PluginVersionManager {
    constructor() {
        this.conventionalCommitRegex = /^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\(.+\))?(!)?:\s(.+)/;
        this.breakingChangeRegex = /BREAKING CHANGE:|^[a-zA-Z]+!:/;
    }

    /**
     * Get commits affecting a specific plugin since last release
     */
    getPluginCommits(pluginPath, lastTag) {
        try {
            const cmd = lastTag 
                ? `git log ${lastTag}..HEAD --pretty=format:"%H|%s|%b" -- ${pluginPath}`
                : `git log --pretty=format:"%H|%s|%b" -- ${pluginPath}`;
            
            const output = execSync(cmd, { encoding: 'utf-8' }).trim();
            if (!output) return [];

            return output.split('\n').map(line => {
                const [hash, subject, body = ''] = line.split('|');
                return { hash, subject, body };
            });
        } catch (error) {
            console.error(`Error getting commits for ${pluginPath}:`, error.message);
            return [];
        }
    }

    /**
     * Analyze commits to determine version bump type
     */
    analyzeCommits(commits) {
        let bump = 'patch'; // Default to patch
        const changes = {
            breaking: [],
            features: [],
            fixes: [],
            other: []
        };

        for (const commit of commits) {
            const fullMessage = `${commit.subject}\n${commit.body}`;
            
            // Check for breaking changes
            if (this.breakingChangeRegex.test(fullMessage)) {
                bump = 'major';
                changes.breaking.push(commit);
                continue;
            }

            // Parse conventional commit
            const match = commit.subject.match(this.conventionalCommitRegex);
            if (match) {
                const [, type, scope, breaking] = match;
                
                if (breaking === '!') {
                    bump = 'major';
                    changes.breaking.push(commit);
                } else if (type === 'feat' && bump !== 'major') {
                    bump = 'minor';
                    changes.features.push(commit);
                } else if (type === 'fix') {
                    changes.fixes.push(commit);
                } else {
                    changes.other.push(commit);
                }
            }
        }

        return { bump, changes };
    }

    /**
     * Get current version from package.json
     */
    getCurrentVersion(pluginPath) {
        const packagePath = path.join(pluginPath, 'package.json');
        try {
            const packageJson = JSON.parse(fs.readFileSync(packagePath, 'utf-8'));
            return packageJson.version || '0.0.0';
        } catch (error) {
            console.warn(`No package.json found for ${pluginPath}, using 0.0.0`);
            return '0.0.0';
        }
    }

    /**
     * Increment version based on bump type
     */
    incrementVersion(currentVersion, bumpType) {
        const [major, minor, patch] = currentVersion.split('.').map(Number);
        
        switch (bumpType) {
            case 'major':
                return `${major + 1}.0.0`;
            case 'minor':
                return `${major}.${minor + 1}.0`;
            case 'patch':
            default:
                return `${major}.${minor}.${patch + 1}`;
        }
    }

    /**
     * Update package.json with new version
     */
    updatePackageVersion(pluginPath, newVersion) {
        const packagePath = path.join(pluginPath, 'package.json');
        try {
            const packageJson = JSON.parse(fs.readFileSync(packagePath, 'utf-8'));
            packageJson.version = newVersion;
            fs.writeFileSync(packagePath, JSON.stringify(packageJson, null, 2) + '\n');
            return true;
        } catch (error) {
            console.error(`Error updating package.json for ${pluginPath}:`, error.message);
            return false;
        }
    }

    /**
     * Get the last release tag for a plugin
     */
    getLastPluginTag(pluginName) {
        try {
            // Look for tags matching this plugin
            const tags = execSync(`git tag -l "${pluginName}-v*" --sort=-version:refname`, { encoding: 'utf-8' })
                .trim()
                .split('\n')
                .filter(Boolean);
            
            return tags.length > 0 ? tags[0] : null;
        } catch (error) {
            return null;
        }
    }

    /**
     * Process a single plugin
     */
    async processPlugin(pluginName, options = {}) {
        const pluginPath = `packages/plugins/${pluginName}`;
        const lastTag = options.lastTag || this.getLastPluginTag(pluginName);
        
        // Get commits since last release
        const commits = this.getPluginCommits(pluginPath, lastTag);
        
        if (commits.length === 0 && !options.force) {
            console.log(`No changes detected for ${pluginName}`);
            return null;
        }

        // Analyze commits for version bump
        const { bump, changes } = this.analyzeCommits(commits);
        
        // Get current version and calculate new version
        const currentVersion = this.getCurrentVersion(pluginPath);
        const newVersion = options.version || this.incrementVersion(currentVersion, bump);

        // Update package.json if requested
        if (options.update) {
            this.updatePackageVersion(pluginPath, newVersion);
        }

        return {
            plugin: pluginName,
            currentVersion,
            newVersion,
            bump,
            changes,
            commits: commits.length,
            lastTag
        };
    }

    /**
     * Generate changelog entries for version
     */
    generateChangelog(changes) {
        const sections = [];

        if (changes.breaking.length > 0) {
            sections.push('### ⚠️ BREAKING CHANGES\n');
            changes.breaking.forEach(commit => {
                sections.push(`* ${commit.subject}`);
            });
            sections.push('');
        }

        if (changes.features.length > 0) {
            sections.push('### ✨ Features\n');
            changes.features.forEach(commit => {
                sections.push(`* ${commit.subject}`);
            });
            sections.push('');
        }

        if (changes.fixes.length > 0) {
            sections.push('### 🐛 Bug Fixes\n');
            changes.fixes.forEach(commit => {
                sections.push(`* ${commit.subject}`);
            });
            sections.push('');
        }

        return sections.join('\n');
    }
}

// CLI interface
if (require.main === module) {
    const args = process.argv.slice(2);
    const command = args[0];
    const manager = new PluginVersionManager();

    switch (command) {
        case 'check': {
            // Check version for a single plugin
            const pluginName = args[1];
            if (!pluginName) {
                console.error('Usage: determine-plugin-version.js check <plugin-name>');
                process.exit(1);
            }

            const result = manager.processPlugin(pluginName);
            if (result) {
                console.log(JSON.stringify(result, null, 2));
            }
            break;
        }

        case 'update': {
            // Update version for a single plugin
            const pluginName = args[1];
            if (!pluginName) {
                console.error('Usage: determine-plugin-version.js update <plugin-name>');
                process.exit(1);
            }

            const result = manager.processPlugin(pluginName, { update: true });
            if (result) {
                console.log(`Updated ${pluginName} from ${result.currentVersion} to ${result.newVersion}`);
                console.log(`Version bump: ${result.bump}`);
                if (result.changes) {
                    console.log('\nChangelog:');
                    console.log(manager.generateChangelog(result.changes));
                }
            }
            break;
        }

        case 'batch': {
            // Process multiple plugins and output JSON
            const plugins = args.slice(1);
            const results = [];

            for (const plugin of plugins) {
                const result = manager.processPlugin(plugin);
                if (result) {
                    results.push(result);
                }
            }

            console.log(JSON.stringify(results, null, 2));
            break;
        }

        case 'update-all': {
            // Update versions for all provided plugins
            const plugins = args.slice(1);
            const results = [];

            for (const plugin of plugins) {
                const result = manager.processPlugin(plugin, { update: true });
                if (result) {
                    results.push(result);
                    console.log(`Updated ${plugin}: ${result.currentVersion} → ${result.newVersion} (${result.bump})`);
                }
            }
            break;
        }

        default:
            console.log(`
Plugin Version Manager - Semantic Versioning based on Conventional Commits

Usage:
  determine-plugin-version.js check <plugin-name>      Check version bump for plugin
  determine-plugin-version.js update <plugin-name>     Update plugin version
  determine-plugin-version.js batch <plugins...>       Check multiple plugins (JSON output)
  determine-plugin-version.js update-all <plugins...>  Update multiple plugins

Examples:
  determine-plugin-version.js check package-manager-apt
  determine-plugin-version.js update desktop-gnome
  determine-plugin-version.js batch package-manager-apt package-manager-dnf
  determine-plugin-version.js update-all desktop-gnome desktop-kde

Conventional Commit Types:
  - feat: New feature (minor bump)
  - fix: Bug fix (patch bump)
  - feat! or fix!: Breaking change (major bump)
  - BREAKING CHANGE: in body (major bump)
  - Other types: patch bump
            `);
            break;
    }
}

module.exports = PluginVersionManager;
