-- Migration: Add dependency tracking and uninstall history tables
-- This migration adds comprehensive dependency tracking and uninstall history support

-- Table for tracking app dependencies
CREATE TABLE IF NOT EXISTS app_dependencies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_name TEXT NOT NULL,
    dependency_name TEXT NOT NULL,
    dependency_type TEXT NOT NULL DEFAULT 'required', -- 'required', 'optional', 'suggested'
    auto_installed BOOLEAN DEFAULT FALSE,
    detected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (app_name) REFERENCES installed_apps(name) ON DELETE CASCADE,
    UNIQUE(app_name, dependency_name)
);

-- Table for tracking what depends on each app (reverse dependencies)
CREATE TABLE IF NOT EXISTS app_dependents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_name TEXT NOT NULL,
    dependent_name TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (app_name) REFERENCES installed_apps(name) ON DELETE CASCADE,
    UNIQUE(app_name, dependent_name)
);

-- Table for uninstall history and rollback support
CREATE TABLE IF NOT EXISTS uninstall_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_name TEXT NOT NULL,
    app_version TEXT,
    install_method TEXT,
    uninstall_date DATETIME DEFAULT CURRENT_TIMESTAMP,
    backup_location TEXT,
    can_restore BOOLEAN DEFAULT TRUE,
    uninstall_method TEXT,
    uninstall_flags TEXT, -- JSON string of flags used
    dependencies_removed TEXT, -- JSON array of dependencies that were removed
    config_files_removed TEXT, -- JSON array of config files that were removed
    data_files_removed TEXT, -- JSON array of data files that were removed
    services_stopped TEXT, -- JSON array of services that were stopped
    package_info TEXT, -- Complete package information for restoration
    rollback_script TEXT, -- Generated script to restore the package
    notes TEXT -- Additional notes about the uninstall
);

-- Table for tracking orphaned packages
CREATE TABLE IF NOT EXISTS orphaned_packages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    package_name TEXT NOT NULL UNIQUE,
    detected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_checked DATETIME DEFAULT CURRENT_TIMESTAMP,
    package_manager TEXT,
    size_kb INTEGER DEFAULT 0,
    description TEXT
);

-- Table for tracking system package protection
CREATE TABLE IF NOT EXISTS protected_packages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    package_name TEXT NOT NULL UNIQUE,
    protection_level TEXT NOT NULL DEFAULT 'critical', -- 'critical', 'important', 'recommended'
    reason TEXT,
    added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    added_by TEXT DEFAULT 'system'
);

-- Indexes for better performance
CREATE INDEX IF NOT EXISTS idx_app_dependencies_app_name ON app_dependencies(app_name);
CREATE INDEX IF NOT EXISTS idx_app_dependencies_dependency_name ON app_dependencies(dependency_name);
CREATE INDEX IF NOT EXISTS idx_app_dependents_app_name ON app_dependents(app_name);
CREATE INDEX IF NOT EXISTS idx_app_dependents_dependent_name ON app_dependents(dependent_name);
CREATE INDEX IF NOT EXISTS idx_uninstall_history_app_name ON uninstall_history(app_name);
CREATE INDEX IF NOT EXISTS idx_uninstall_history_date ON uninstall_history(uninstall_date);
CREATE INDEX IF NOT EXISTS idx_orphaned_packages_name ON orphaned_packages(package_name);
CREATE INDEX IF NOT EXISTS idx_protected_packages_name ON protected_packages(package_name);

-- Insert default protected packages
INSERT OR IGNORE INTO protected_packages (package_name, protection_level, reason, added_by) VALUES
('kernel', 'critical', 'Linux kernel - system will not boot without it', 'system'),
('systemd', 'critical', 'System and service manager', 'system'),
('glibc', 'critical', 'GNU C Library - required by most programs', 'system'),
('bash', 'critical', 'Bourne Again Shell - default shell', 'system'),
('coreutils', 'critical', 'Core utilities (ls, cp, mv, etc.)', 'system'),
('sudo', 'critical', 'Superuser do - required for administrative tasks', 'system'),
('openssh', 'important', 'OpenSSH server and client', 'system'),
('networkmanager', 'important', 'Network management daemon', 'system'),
('dbus', 'important', 'D-Bus message bus system', 'system'),
('udev', 'important', 'Device manager for the Linux kernel', 'system');