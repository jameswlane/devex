package main

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// HandleAliases installs useful Git aliases to improve workflow efficiency
func (p *GitPlugin) HandleAliases(ctx context.Context, args []string) error {
	// Check if this is a "set" command for custom aliases
	if len(args) >= 3 && args[0] == "set" {
		return p.setCustomAlias(ctx, args[1], args[2])
	}

	// Default behavior: install predefined aliases
	fmt.Println("Installing Git aliases...")

	aliases := p.GetGitAliases()

	// Install each alias
	installed := 0
	for alias, command := range aliases {
		if err := sdk.ExecCommandWithContext(ctx, false, "git", "config", "--global", fmt.Sprintf("alias.%s", alias), command); err != nil {
			fmt.Printf("Warning: failed to install alias '%s': %v\n", alias, err)
		} else {
			installed++
		}
	}

	fmt.Printf("\nInstalled %d git aliases!\n", installed)
	fmt.Println("Use 'git la' to list all aliases")

	return nil
}

// setCustomAlias sets a custom git alias
func (p *GitPlugin) setCustomAlias(ctx context.Context, aliasName, aliasCommand string) error {
	// Additional validation for alias names and commands
	if err := p.validateAliasName(aliasName); err != nil {
		return err
	}

	if err := p.validateAliasCommand(aliasCommand); err != nil {
		return err
	}

	return sdk.ExecCommandWithContext(ctx, false, "git", "config", "--global", fmt.Sprintf("alias.%s", aliasName), aliasCommand)
}

// validateAliasName validates that an alias name is safe
func (p *GitPlugin) validateAliasName(name string) error {
	// Alias names should only contain letters, numbers, and hyphens
	if name == "" {
		return fmt.Errorf("alias name cannot be empty")
	}

	// Check for dangerous characters that were already caught by general validation
	// This is additional semantic validation
	if strings.Contains(name, " ") {
		return fmt.Errorf("alias name cannot contain spaces")
	}

	return nil
}

// validateAliasCommand validates that an alias command is safe
func (p *GitPlugin) validateAliasCommand(command string) error {
	if command == "" {
		return fmt.Errorf("alias command cannot be empty")
	}

	// Additional validation can be added here if needed
	return nil
}

// GetGitAliases returns a comprehensive set of useful Git aliases
func (p *GitPlugin) GetGitAliases() map[string]string {
	return map[string]string{
		// Basic shortcuts
		"st": "status",
		"co": "checkout",
		"br": "branch",
		"ci": "commit",
		"cp": "cherry-pick",
		"cl": "clone",
		"dc": "diff --cached",

		// Staging operations
		"unstage":  "reset HEAD --",
		"assume":   "update-index --assume-unchanged",
		"unassume": "update-index --no-assume-unchanged",
		"assumed":  "!git ls-files -v | grep ^h | cut -c 3-",

		// Log and history
		"last": "log -1 HEAD",
		"lg":   "log --oneline --graph --decorate",
		"lga":  "log --oneline --graph --decorate --all",
		"ll":   "log --pretty=format:'%C(yellow)%h%Cred%d %Creset%s%Cblue [%cn]' --decorate --numstat",
		"ls":   "log --pretty=format:'%C(yellow)%h%Cred%d %Creset%s%Cblue [%cn]' --decorate",
		"lds":  "log --pretty=format:'%C(yellow)%h %ad%Cred%d %Creset%s%Cblue [%cn]' --decorate --date=short",
		"le":   "log --oneline --decorate",

		// Diff operations
		"dl":    "!git ll -1",
		"dlc":   "diff --cached HEAD^",
		"dr":    "!f() { git diff \"$1\"^..\"$1\"; }; f",
		"lc":    "!f() { git ll \"$1\"^..\"$1\"; }; f",
		"diffr": "!f() { git diff \"$1\"^..\"$1\"; }; f",

		// Reset operations
		"r":   "reset",
		"r1":  "reset HEAD^",
		"r2":  "reset HEAD^^",
		"rh":  "reset --hard",
		"rh1": "reset HEAD^ --hard",
		"rh2": "reset HEAD^^ --hard",

		// Search and find
		"f":   "!git ls-files | grep -i",
		"gra": "!f() { A=$(pwd) && TOPLEVEL=$(git rev-parse --show-toplevel) && cd $TOPLEVEL && git grep --full-name -In $1 | xargs -I{} echo $TOPLEVEL/{} && cd $A; }; f",

		// Branch management
		"done": "!f() { git branch | grep \"$1\" | cut -c 3- | grep -v done | xargs -I{} git branch -m {} done-{}; }; f",

		// Merge conflict resolution
		"ours":   "!f() { git co --ours $@ && git add $@; }; f",
		"theirs": "!f() { git co --theirs $@ && git add $@; }; f",

		// Utility
		"la":     "!git config -l | grep alias | cut -c 7-",
		"visual": "!gitk",

		// SVN integration (for legacy projects)
		"svnr": "svn rebase",
		"svnd": "svn dcommit",
		"svnl": "svn log --oneline --show-commit",
	}
}
