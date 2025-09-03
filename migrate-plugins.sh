#!/bin/bash

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 DevEx Plugin Migration to Turborepo${NC}"
echo "============================================="

# Get list of all plugins that need migration (exclude the 4 we already did)
plugins_to_migrate=($(ls -1 packages/plugins/ | grep -v -E "^(package-manager-apt|desktop-gnome|system-setup)$"))

echo -e "${YELLOW}Found ${#plugins_to_migrate[@]} plugins to migrate:${NC}"
for plugin in "${plugins_to_migrate[@]}"; do
    echo "  - $plugin"
done

echo ""
echo -e "${GREEN}Starting migration...${NC}"

for plugin in "${plugins_to_migrate[@]}"; do
    echo -e "${YELLOW}Migrating $plugin...${NC}"
    
    # 1. Create new directory if it doesn't exist
    if [ ! -d "packages/$plugin" ]; then
        mkdir -p "packages/$plugin"
        echo "  ✅ Created packages/$plugin/"
    else
        echo "  ℹ️ packages/$plugin/ already exists"
    fi
    
    # 2. Copy all files except existing ones we might have created
    cp -r "packages/plugins/$plugin/"* "packages/$plugin/" 2>/dev/null || true
    echo "  ✅ Copied files"
    
    # 3. Check if package.json exists and update it
    if [ -f "packages/$plugin/package.json" ]; then
        echo "  ✅ package.json exists, updating..."
        
        # Update package name (remove plugin- prefix)
        sed -i "s|\"@devex/plugin-|\"@devex/|g" "packages/$plugin/package.json"
        
        # Add private: true if not present
        if ! grep -q '"private":' "packages/$plugin/package.json"; then
            sed -i '/"name":/ a\	"private": true,' "packages/$plugin/package.json"
        fi
        
        # Update scripts to use direct Go commands instead of task
        sed -i 's|"build": "task build"|"build": "go build -o devex-plugin-'"$plugin"' main.go"|g' "packages/$plugin/package.json"
        sed -i 's|"test": "task test"|"test": "go test ./..."|g' "packages/$plugin/package.json"
        sed -i 's|"lint": "task lint"|"lint": "golangci-lint run"|g' "packages/$plugin/package.json"
        
        # Add dev and mod scripts if not present
        if ! grep -q '"dev":' "packages/$plugin/package.json"; then
            sed -i '/"build":/ a\		"dev": "go build -race -o devex-plugin-'"$plugin"' main.go",' "packages/$plugin/package.json"
        fi
        if ! grep -q '"mod":' "packages/$plugin/package.json"; then
            sed -i '/"lint":/ a\		"mod": "go mod tidy"' "packages/$plugin/package.json"
        fi
        
        # Add workspace dependency to plugin-sdk if not present
        if ! grep -q '"@devex/plugin-sdk":' "packages/$plugin/package.json"; then
            # Find the right place to add dependencies
            if grep -q '"dependencies":' "packages/$plugin/package.json"; then
                # Add to existing dependencies
                sed -i '/"dependencies": {/ a\		"@devex/plugin-sdk": "workspace:*",' "packages/$plugin/package.json"
            else
                # Create dependencies section after scripts
                sed -i '/},$/N; s|},\n|},\n\t"dependencies": {\n\t\t"@devex/plugin-sdk": "workspace:*"\n\t},\n|' "packages/$plugin/package.json"
            fi
        fi
        
        # Update repository directory path
        sed -i "s|packages/plugins/$plugin|packages/$plugin|g" "packages/$plugin/package.json"
        
        echo "  ✅ Updated package.json"
    else
        echo "  ❌ No package.json found for $plugin"
        continue
    fi
    
    # 4. Update go.mod if it exists
    if [ -f "packages/$plugin/go.mod" ]; then
        # Update module name
        sed -i "s|github.com/jameswlane/devex/packages/plugins/|github.com/jameswlane/devex/packages/|g" "packages/$plugin/go.mod"
        
        # Update plugin-sdk dependency and replace directive
        sed -i "s|github.com/jameswlane/devex/packages/shared/plugin-sdk|github.com/jameswlane/devex/packages/plugin-sdk|g" "packages/$plugin/go.mod"
        sed -i "s|../../shared/plugin-sdk|../plugin-sdk|g" "packages/$plugin/go.mod"
        
        echo "  ✅ Updated go.mod"
    fi
    
    # 5. Update Go import statements in main.go
    if [ -f "packages/$plugin/main.go" ]; then
        sed -i "s|github.com/jameswlane/devex/packages/shared/plugin-sdk|github.com/jameswlane/devex/packages/plugin-sdk|g" "packages/$plugin/main.go"
        echo "  ✅ Updated Go imports"
    fi
    
    # 6. Update any other .go files that might import the SDK
    find "packages/$plugin" -name "*.go" -exec sed -i "s|github.com/jameswlane/devex/packages/shared/plugin-sdk|github.com/jameswlane/devex/packages/plugin-sdk|g" {} \;
    
    echo -e "  ✅ ${GREEN}$plugin migration complete${NC}"
    echo ""
done

echo -e "${GREEN}🎉 Migration completed!${NC}"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Run: pnpm install"
echo "2. Test: pnpm turbo build --filter=\"@devex/*\""
echo "3. Remove old packages/plugins directory when satisfied"
echo "4. Update CI workflows to use Turborepo"