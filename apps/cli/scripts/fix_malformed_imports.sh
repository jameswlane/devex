#!/bin/bash

# Fix malformed imports in test suite files
# Replace 'testhelper". "github' with 'testhelper"\n\t. "github'

cd /data/GitHub/jameswlane/devex/apps/cli

# Find and fix all malformed imports
find pkg/ -name "*_suite_test.go" -type f -exec sed -i 's/"github.com\/jameswlane\/devex\/pkg\/testhelper"\. "github/"github.com\/jameswlane\/devex\/pkg\/testhelper"\n\t. "github/g' {} \;

echo "Fixed malformed imports in test suite files"