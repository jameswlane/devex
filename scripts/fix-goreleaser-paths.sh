#!/bin/bash

# Fix all plugin paths in .goreleaser.yml by removing the incorrect /plugins/ directory
sed -i 's|packages/plugins/|packages/|g' .goreleaser.yml

echo "Fixed all plugin paths in .goreleaser.yml"