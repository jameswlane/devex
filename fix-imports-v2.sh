#!/bin/bash

# Script to fix import paths after reorganizing packages from pkg/ to internal/
# This version removes references to the old installer implementations which are now plugins

set -e

cd /data/GitHub/jameswlane/devex/apps/cli

echo "Fixing import paths and removing legacy installer imports..."

# Fix core imports
find . -name "*.go" -type f -exec sed -i \
  -e 's|github.com/jameswlane/devex/pkg/backup|github.com/jameswlane/devex/apps/cli/internal/backup|g' \
  -e 's|github.com/jameswlane/devex/pkg/cache|github.com/jameswlane/devex/apps/cli/internal/cache|g' \
  -e 's|github.com/jameswlane/devex/pkg/commands|github.com/jameswlane/devex/apps/cli/internal/commands|g' \
  -e 's|github.com/jameswlane/devex/pkg/common|github.com/jameswlane/devex/apps/cli/internal/common|g' \
  -e 's|github.com/jameswlane/devex/pkg/config|github.com/jameswlane/devex/apps/cli/internal/config|g' \
  -e 's|github.com/jameswlane/devex/pkg/constants|github.com/jameswlane/devex/apps/cli/internal/constants|g' \
  -e 's|github.com/jameswlane/devex/pkg/datastore|github.com/jameswlane/devex/apps/cli/internal/datastore|g' \
  -e 's|github.com/jameswlane/devex/pkg/db|github.com/jameswlane/devex/apps/cli/internal/db|g' \
  -e 's|github.com/jameswlane/devex/pkg/errors|github.com/jameswlane/devex/apps/cli/internal/errors|g' \
  -e 's|github.com/jameswlane/devex/pkg/fs|github.com/jameswlane/devex/apps/cli/internal/fs|g' \
  -e 's|github.com/jameswlane/devex/pkg/help|github.com/jameswlane/devex/apps/cli/internal/help|g' \
  -e 's|github.com/jameswlane/devex/pkg/installer|github.com/jameswlane/devex/apps/cli/internal/installer|g' \
  -e 's|github.com/jameswlane/devex/pkg/installers|github.com/jameswlane/devex/apps/cli/internal/installers|g' \
  -e 's|github.com/jameswlane/devex/pkg/log|github.com/jameswlane/devex/apps/cli/internal/log|g' \
  -e 's|github.com/jameswlane/devex/pkg/metrics|github.com/jameswlane/devex/apps/cli/internal/metrics|g' \
  -e 's|github.com/jameswlane/devex/pkg/mocks|github.com/jameswlane/devex/apps/cli/internal/mocks|g' \
  -e 's|github.com/jameswlane/devex/pkg/performance|github.com/jameswlane/devex/apps/cli/internal/performance|g' \
  -e 's|github.com/jameswlane/devex/pkg/platform|github.com/jameswlane/devex/apps/cli/internal/platform|g' \
  -e 's|github.com/jameswlane/devex/pkg/progress|github.com/jameswlane/devex/apps/cli/internal/progress|g' \
  -e 's|github.com/jameswlane/devex/pkg/recovery|github.com/jameswlane/devex/apps/cli/internal/recovery|g' \
  -e 's|github.com/jameswlane/devex/pkg/security|github.com/jameswlane/devex/apps/cli/internal/security|g' \
  -e 's|github.com/jameswlane/devex/pkg/sysmetrics|github.com/jameswlane/devex/apps/cli/internal/sysmetrics|g' \
  -e 's|github.com/jameswlane/devex/pkg/system|github.com/jameswlane/devex/apps/cli/internal/system|g' \
  -e 's|github.com/jameswlane/devex/pkg/templates|github.com/jameswlane/devex/apps/cli/internal/templates|g' \
  -e 's|github.com/jameswlane/devex/pkg/tui|github.com/jameswlane/devex/apps/cli/internal/tui|g' \
  -e 's|github.com/jameswlane/devex/pkg/types|github.com/jameswlane/devex/apps/cli/internal/types|g' \
  -e 's|github.com/jameswlane/devex/pkg/undo|github.com/jameswlane/devex/apps/cli/internal/undo|g' \
  -e 's|github.com/jameswlane/devex/pkg/utils|github.com/jameswlane/devex/apps/cli/internal/utils|g' \
  -e 's|github.com/jameswlane/devex/pkg/version|github.com/jameswlane/devex/apps/cli/internal/version|g' \
  {} \;

echo "Updated import paths from pkg/ to internal/"

# Fix plugins that reference the old non-plugin packages 
echo "Removing old package references from plugins..."

find packages/plugins -name "*.go" -type f -exec sed -i \
  -e 's|github.com/jameswlane/devex/pkg/fonts|.|g' \
  -e 's|github.com/jameswlane/devex/pkg/gitconfig|.|g' \
  -e 's|github.com/jameswlane/devex/pkg/gnome|.|g' \
  -e 's|github.com/jameswlane/devex/pkg/shell|.|g' \
  -e 's|github.com/jameswlane/devex/pkg/stackdetector|.|g' \
  -e 's|github.com/jameswlane/devex/pkg/systemsetup|.|g' \
  -e 's|github.com/jameswlane/devex/pkg/themes|.|g' \
  {} \;

echo "Plugin import fixes completed!"