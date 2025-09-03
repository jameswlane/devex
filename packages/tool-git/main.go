package main

// Build timestamp: 2025-09-03 17:41:19

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// GitPlugin implements the Git configuration plugin
type GitPlugin struct {
	*sdk.BasePlugin
}

// NewGitPlugin creates a new Git plugin
func NewGitPlugin() *GitPlugin {
	info := sdk.PluginInfo{
		Name:        "tool-git",
		Version:     version,
		Description: "Git configuration and alias management",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"git", "vcs", "development"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "config",
				Description: "Configure Git settings",
				Usage:       "Set up Git user, email, and global configuration",
			},
			{
				Name:        "aliases",
				Description: "Install Git aliases",
				Usage:       "Install useful Git aliases from configuration",
			},
			{
				Name:        "status",
				Description: "Show Git configuration status",
				Usage:       "Display current Git configuration",
			},
		},
	}

	return &GitPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

// Execute handles command execution
func (p *GitPlugin) Execute(command string, args []string) error {
	// Ensure git is available
	if !sdk.CommandExists("git") {
		return fmt.Errorf("git is not installed on this system")
	}

	switch command {
	case "config":
		return p.handleConfig(args)
	case "aliases":
		return p.handleAliases(args)
	case "status":
		return p.handleStatus(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *GitPlugin) handleConfig(args []string) error {
	fmt.Println("Configuring Git...")

	// Check for command line arguments for name and email
	var fullName, email string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 < len(args) {
				fullName = args[i+1]
				i++
			}
		case "--email":
			if i+1 < len(args) {
				email = args[i+1]
				i++
			}
		}
	}

	// If not provided via args, get current values or prompt
	if fullName == "" {
		currentName, _ := sdk.RunCommand("git", "config", "--global", "user.name")
		currentName = strings.TrimSpace(currentName)
		if currentName != "" {
			fmt.Printf("Current user name: %s\n", currentName)
			fullName = currentName
		} else {
			fmt.Println("No git user name configured")
			if len(args) == 0 {
				fmt.Println("Use: git config --name \"Your Name\" --email \"your@email.com\"")
				return nil
			}
		}
	}

	if email == "" {
		currentEmail, _ := sdk.RunCommand("git", "config", "--global", "user.email")
		currentEmail = strings.TrimSpace(currentEmail)
		if currentEmail != "" {
			fmt.Printf("Current user email: %s\n", currentEmail)
			email = currentEmail
		} else {
			fmt.Println("No git user email configured")
			if len(args) == 0 {
				fmt.Println("Use: git config --name \"Your Name\" --email \"your@email.com\"")
				return nil
			}
		}
	}

	// Set git configuration
	if fullName != "" {
		if err := sdk.ExecCommand(false, "git", "config", "--global", "user.name", fullName); err != nil {
			return fmt.Errorf("failed to set git user name: %w", err)
		}
		fmt.Printf("Set git user name: %s\n", fullName)
	}

	if email != "" {
		if err := sdk.ExecCommand(false, "git", "config", "--global", "user.email", email); err != nil {
			return fmt.Errorf("failed to set git user email: %w", err)
		}
		fmt.Printf("Set git user email: %s\n", email)
	}

	// Set default branch name
	if err := sdk.ExecCommand(false, "git", "config", "--global", "init.defaultBranch", "main"); err != nil {
		fmt.Printf("Warning: failed to set default branch name: %v\n", err)
	} else {
		fmt.Println("Set default branch name to 'main'")
	}

	// Set other useful defaults
	configs := map[string]string{
		"core.editor":         "vim",
		"color.ui":            "auto",
		"pull.rebase":         "false",
		"push.default":        "simple",
		"credential.helper":   "cache --timeout=3600",
		"merge.conflictstyle": "diff3",
		"diff.colorMoved":     "default",
		"fetch.prune":         "true",
	}

	for key, value := range configs {
		if err := sdk.ExecCommand(false, "git", "config", "--global", key, value); err != nil {
			fmt.Printf("Warning: failed to set %s: %v\n", key, err)
		}
	}

	fmt.Println("\nGit configuration complete!")
	return nil
}

func (p *GitPlugin) handleAliases(args []string) error {
	fmt.Println("Installing Git aliases...")

	// Define useful git aliases
	aliases := map[string]string{
		"st":       "status",
		"co":       "checkout",
		"br":       "branch",
		"ci":       "commit",
		"unstage":  "reset HEAD --",
		"last":     "log -1 HEAD",
		"visual":   "!gitk",
		"lg":       "log --oneline --graph --decorate",
		"lga":      "log --oneline --graph --decorate --all",
		"ll":       "log --pretty=format:'%C(yellow)%h%Cred%d %Creset%s%Cblue [%cn]' --decorate --numstat",
		"ls":       "log --pretty=format:'%C(yellow)%h%Cred%d %Creset%s%Cblue [%cn]' --decorate",
		"lds":      "log --pretty=format:'%C(yellow)%h %ad%Cred%d %Creset%s%Cblue [%cn]' --decorate --date=short",
		"le":       "log --oneline --decorate",
		"dl":       "!git ll -1",
		"dlc":      "diff --cached HEAD^",
		"dr":       "!f() { git diff \"$1\"^..\"$1\"; }; f",
		"lc":       "!f() { git ll \"$1\"^..\"$1\"; }; f",
		"diffr":    "!f() { git diff \"$1\"^..\"$1\"; }; f",
		"f":        "!git ls-files | grep -i",
		"gra":      "!f() { A=$(pwd) && TOPLEVEL=$(git rev-parse --show-toplevel) && cd $TOPLEVEL && git grep --full-name -In $1 | xargs -I{} echo $TOPLEVEL/{} && cd $A; }; f",
		"la":       "!git config -l | grep alias | cut -c 7-",
		"done":     "!f() { git branch | grep \"$1\" | cut -c 3- | grep -v done | xargs -I{} git branch -m {} done-{}; }; f",
		"assume":   "update-index --assume-unchanged",
		"unassume": "update-index --no-assume-unchanged",
		"assumed":  "!git ls-files -v | grep ^h | cut -c 3-",
		"ours":     "!f() { git co --ours $@ && git add $@; }; f",
		"theirs":   "!f() { git co --theirs $@ && git add $@; }; f",
		"cp":       "cherry-pick",
		"cl":       "clone",
		"dc":       "diff --cached",
		"r":        "reset",
		"r1":       "reset HEAD^",
		"r2":       "reset HEAD^^",
		"rh":       "reset --hard",
		"rh1":      "reset HEAD^ --hard",
		"rh2":      "reset HEAD^^ --hard",
		"svnr":     "svn rebase",
		"svnd":     "svn dcommit",
		"svnl":     "svn log --oneline --show-commit",
	}

	// Install each alias
	installed := 0
	for alias, command := range aliases {
		if err := sdk.ExecCommand(false, "git", "config", "--global", fmt.Sprintf("alias.%s", alias), command); err != nil {
			fmt.Printf("Warning: failed to install alias '%s': %v\n", alias, err)
		} else {
			installed++
		}
	}

	fmt.Printf("\nInstalled %d git aliases!\n", installed)
	fmt.Println("Use 'git la' to list all aliases")

	return nil
}

func (p *GitPlugin) handleStatus(args []string) error {
	fmt.Println("Git Configuration Status:")

	// Show git version
	output, err := sdk.RunCommand("git", "--version")
	if err != nil {
		return fmt.Errorf("failed to get git version: %w", err)
	}
	fmt.Printf("Version: %s\n", strings.TrimSpace(output))

	// Show user configuration
	user, err := sdk.RunCommand("git", "config", "--global", "user.name")
	if err == nil {
		fmt.Printf("User Name: %s\n", strings.TrimSpace(user))
	}

	email, err := sdk.RunCommand("git", "config", "--global", "user.email")
	if err == nil {
		fmt.Printf("User Email: %s\n", strings.TrimSpace(email))
	}

	return nil
}

func main() {
	plugin := NewGitPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
