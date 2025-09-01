#!/bin/bash

# Add log suppression calls to test suite files that are missing them

cd /data/GitHub/jameswlane/devex/apps/cli

# List of files that need the suppression call added
files=(
    "pkg/shell/shell_suite_test.go"
    "pkg/tui/tui_suite_test.go"
    "pkg/fonts/fonts_suite_test.go"
    "pkg/db/db_suite_test.go"
    "pkg/errors/errors_suite_test.go"
    "pkg/systemsetup/systemsetup_suite_test.go"
    "pkg/themes/themes_suite_test.go"
    "pkg/performance/performance_suite_test.go"
    "pkg/sysmetrics/sysmetrics_suite_test.go"
    "pkg/system/system_suite_test.go"
    "pkg/gnome/gnome_suite_test.go"
    "pkg/gitconfig/gitconfig_suite_test.go"
    "pkg/progress/progress_suite_test.go"
    "pkg/cache/cache_suite_test.go"
    "pkg/mocks/mocks_suite_test.go"
    "pkg/installers/eopkg/eopkg_suite_test.go"
    "pkg/installers/curlpipe/curlpipe_suite_test.go"
    "pkg/installers/nixpkgs/nixpkgs_suite_test.go"
    "pkg/installers/apt/apt_suite_test.go"
    "pkg/installers/pacman/pacman_suite_test.go"
    "pkg/installers/nixflake/nixflake_suite_test.go"
    "pkg/installers/rpm/rpm_suite_test.go"
    "pkg/installers/dnf/dnf_suite_test.go"
    "pkg/installers/snap/snap_suite_test.go"
    "pkg/installers/pip/pip_suite_test.go"
    "pkg/installers/zypper/zypper_suite_test.go"
    "pkg/installers/flatpak/flatpak_suite_test.go"
    "pkg/installers/brew/brew_suite_test.go"
    "pkg/installers/appimage/appimage_suite_test.go"
    "pkg/installers/xbps/xbps_suite_test.go"
    "pkg/installers/emerge/emerge_suite_test.go"
    "pkg/installers/utilities/utilities_suite_test.go"
    "pkg/utils/utils_suite_test.go"
    "pkg/security/security_suite_test.go"
    "pkg/types/types_suite_test.go"
    "pkg/log/log_suite_test.go"
    "pkg/fs/fs_suite_test.go"
    "pkg/common/common_suite_test.go"
)

suppression_code='
// Set up test logging suppression for all tests in this suite
var _ = BeforeEach(func() {
	testhelper.SuppressLogs()
})'

for file in "${files[@]}"; do
    if [ -f "$file" ]; then
        # Check if the file already has the suppression code
        if ! grep -q "testhelper.SuppressLogs()" "$file"; then
            # Check if it has the old SetupTestLogging call
            if grep -q "testhelper.SetupTestLogging()" "$file"; then
                # Replace the old call with the new one
                sed -i 's/var _ = testhelper.SetupTestLogging()/var _ = BeforeEach(func() {\
	testhelper.SuppressLogs()\
})/g' "$file"
            else
                # Add the suppression code at the end of the file
                echo "$suppression_code" >> "$file"
            fi
            echo "Updated: $file"
        else
            echo "Already has suppression: $file"
        fi
    else
        echo "File not found: $file"
    fi
done

echo "Done!"