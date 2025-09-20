-- Migration rollback: Remove dependency tracking and uninstall history tables

-- Drop indexes first
DROP INDEX IF EXISTS idx_protected_packages_name;
DROP INDEX IF EXISTS idx_orphaned_packages_name;
DROP INDEX IF EXISTS idx_uninstall_history_date;
DROP INDEX IF EXISTS idx_uninstall_history_app_name;
DROP INDEX IF EXISTS idx_app_dependents_dependent_name;
DROP INDEX IF EXISTS idx_app_dependents_app_name;
DROP INDEX IF EXISTS idx_app_dependencies_dependency_name;
DROP INDEX IF EXISTS idx_app_dependencies_app_name;

-- Drop tables
DROP TABLE IF EXISTS protected_packages;
DROP TABLE IF EXISTS orphaned_packages;
DROP TABLE IF EXISTS uninstall_history;
DROP TABLE IF EXISTS app_dependents;
DROP TABLE IF EXISTS app_dependencies;