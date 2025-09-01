#!/bin/bash

# Fix missing testhelper imports in test suite files

cd /data/GitHub/jameswlane/devex/apps/cli

# Files that need the testhelper import
files=(
    "pkg/common/common_suite_test.go"
    "pkg/fs/fs_suite_test.go"
    "pkg/log/log_suite_test.go"
    "pkg/types/types_suite_test.go"
    "pkg/security/security_suite_test.go"
    "pkg/installers/utilities/utilities_suite_test.go"
)

for file in "${files[@]}"; do
    if [ -f "$file" ]; then
        # Check if testhelper import is missing
        if ! grep -q "github.com/jameswlane/devex/pkg/testhelper" "$file"; then
            # Add the import after "testing" import
            sed -i '/^import (/,/^)/ {
                /"testing"/ {
                    a\\
\t"github.com/jameswlane/devex/pkg/testhelper"
                }
            }' "$file"
            echo "Fixed import in: $file"
        else
            echo "Import already present in: $file"
        fi
    else
        echo "File not found: $file"
    fi
done

echo "Done!"