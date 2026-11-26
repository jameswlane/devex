package commands

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/agentinit"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type agentInitOptions struct {
	projectDir string
	overwrite  bool
	mcp        bool
}

type plannedManagedFile struct {
	relPath string
	content []byte
}

func newAgentInitCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var opts agentInitOptions

	cmd := &cobra.Command{
		Use:          "init",
		Short:        "Scaffold project coding-agent instruction files",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Detect project languages, task runners, and test/lint commands offline and
scaffold project-level coding-agent files in the target repository:

  AGENTS.md   Canonical project instructions (read natively by Codex, OpenCode, and Cursor)
  CLAUDE.md   Thin shim importing @AGENTS.md for Claude Code
  GEMINI.md   Thin shim importing @./AGENTS.md for Gemini CLI
  .gitignore  Appends missing agent-local entries (.claude/settings.local.json, CLAUDE.local.md)
  .mcp.json   Optional (--mcp) Claude Code project-scope MCP servers rendered from agents.yaml

Existing files with identical content are reported unchanged. Differing files
are skipped unless --overwrite is given, in which case the previous file is
backed up beside it as <name>.devex-backup-<unixnano>.

Honors the global --dry-run flag and DEVEX_DRY_RUN environment variable (zero writes).`,
		Example: `  # Scaffold agent files in the current directory
  devex agent init

  # Preview what would be created or updated without writing files
  devex agent init --dry-run

  # Target another directory and include Claude Code project-scope .mcp.json
  devex agent init --project-dir ./my-service --mcp

  # Replace differing scaffolded files while keeping timestamped backups
  devex agent init --overwrite`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dryRun := viper.GetBool("dry-run")
			return runAgentInit(cmd, settings, opts, dryRun)
		},
	}

	cmd.Flags().StringVar(&opts.projectDir, "project-dir", "", "Project root directory (default: current working directory)")
	cmd.Flags().BoolVar(&opts.overwrite, "overwrite", false, "Replace differing existing files and keep timestamped backups")
	cmd.Flags().BoolVar(&opts.mcp, "mcp", false, "Render Claude Code project-scope .mcp.json from agents.yaml")
	return cmd
}

func runAgentInit(cmd *cobra.Command, settings config.CrossPlatformSettings, opts agentInitOptions, dryRun bool) error {
	if err := cmd.Context().Err(); err != nil {
		return err
	}

	var (
		mcpContent []byte
		hasMCP     bool
	)
	if opts.mcp {
		var err error
		mcpContent, hasMCP, err = agentinit.RenderClaudeMCPJSON(settings.Agents)
		if err != nil {
			return err
		}
	}

	projectDir, err := resolveAgentInitProjectDir(opts.projectDir)
	if err != nil {
		return err
	}

	root, err := os.OpenRoot(projectDir)
	if err != nil {
		return fmt.Errorf("open project directory %q: %w", projectDir, err)
	}
	defer func() { _ = root.Close() }()

	info := agentinit.Detect(root.FS(), filepath.Base(projectDir))

	agentsMD, err := agentinit.RenderAgentsMD(info)
	if err != nil {
		return err
	}
	claudeMD, err := agentinit.RenderClaudeMD()
	if err != nil {
		return err
	}
	geminiMD, err := agentinit.RenderGeminiMD()
	if err != nil {
		return err
	}

	managed := []plannedManagedFile{
		{relPath: "AGENTS.md", content: agentsMD},
		{relPath: "CLAUDE.md", content: claudeMD},
		{relPath: "GEMINI.md", content: geminiMD},
	}
	if hasMCP {
		managed = append(managed, plannedManagedFile{relPath: ".mcp.json", content: mcpContent})
	}

	// Validate all planned destinations before writing anything.
	for _, f := range managed {
		if err := validateAgentInitPath(root, f.relPath); err != nil {
			return err
		}
	}
	if err := validateAgentInitPath(root, ".gitignore"); err != nil {
		return err
	}

	if err := cmd.Context().Err(); err != nil {
		return err
	}

	for _, f := range managed {
		if err := applyManagedInitFile(cmd, root, projectDir, f, opts.overwrite, dryRun); err != nil {
			return err
		}
	}

	if opts.mcp && !hasMCP {
		cmd.Printf("Skipped (.mcp.json): no MCP servers configured for Claude Code (agents.enabled must include claude)\n")
	}

	return applyGitignoreInitFile(cmd, root, projectDir, dryRun)
}

func resolveAgentInitProjectDir(projectDir string) (string, error) {
	var dir string
	var err error
	if projectDir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve current working directory: %w", err)
		}
	} else {
		dir, err = filepath.Abs(projectDir)
		if err != nil {
			return "", fmt.Errorf("resolve project directory %q: %w", projectDir, err)
		}
	}
	dir = filepath.Clean(dir)

	info, err := os.Stat(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "", fmt.Errorf("project directory %q does not exist", dir)
	case err != nil:
		return "", fmt.Errorf("inspect project directory %q: %w", dir, err)
	case !info.IsDir():
		return "", fmt.Errorf("project directory %q is not a directory", dir)
	}
	return dir, nil
}

func validateAgentInitPath(root *os.Root, relPath string) error {
	parts := strings.Split(relPath, "/")
	current := ""
	for i, part := range parts {
		if current == "" {
			current = part
		} else {
			current += "/" + part
		}
		info, err := root.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect destination %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("destination %q contains a symlink at %q", relPath, current)
		}
		isLast := i == len(parts)-1
		if isLast {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("destination %q is not a regular file", relPath)
			}
		} else if !info.IsDir() {
			return fmt.Errorf("destination path component %q is not a directory", current)
		}
	}
	return nil
}

func applyManagedInitFile(cmd *cobra.Command, root *os.Root, projectDir string, file plannedManagedFile, overwrite, dryRun bool) error {
	absPath := filepath.Join(projectDir, filepath.FromSlash(file.relPath))

	existing, err := root.ReadFile(file.relPath)
	exists := true
	if errors.Is(err, os.ErrNotExist) {
		exists = false
	} else if err != nil {
		return fmt.Errorf("read existing %q: %w", absPath, err)
	}

	if !exists {
		if dryRun {
			cmd.Printf("Would create: %s\n", absPath)
			return nil
		}
		if err := root.WriteFile(file.relPath, file.content, 0o644); err != nil {
			return fmt.Errorf("write %q: %w", absPath, err)
		}
		cmd.Printf("Created: %s\n", absPath)
		return nil
	}

	if bytes.Equal(existing, file.content) {
		cmd.Printf("Unchanged: %s\n", absPath)
		return nil
	}

	if !overwrite {
		if dryRun {
			cmd.Printf("Would skip (exists): %s (use --overwrite to replace)\n", absPath)
			return nil
		}
		cmd.Printf("Skipped (exists): %s (use --overwrite to replace)\n", absPath)
		return nil
	}

	if dryRun {
		cmd.Printf("Would replace: %s (existing file would be backed up)\n", absPath)
		return nil
	}

	backupRel := fmt.Sprintf("%s.devex-backup-%d", file.relPath, time.Now().UnixNano())
	backupAbs := filepath.Join(projectDir, filepath.FromSlash(backupRel))
	if err := root.Rename(file.relPath, backupRel); err != nil {
		return fmt.Errorf("back up existing %q: %w", absPath, err)
	}
	if err := root.WriteFile(file.relPath, file.content, 0o644); err != nil {
		if restoreErr := root.Rename(backupRel, file.relPath); restoreErr != nil {
			return fmt.Errorf("write %q: %w; restore backup %q manually: %w", absPath, err, backupAbs, restoreErr)
		}
		return fmt.Errorf("write %q: %w", absPath, err)
	}
	cmd.Printf("Replaced: %s\nBackup: %s\n", absPath, backupAbs)
	return nil
}

func applyGitignoreInitFile(cmd *cobra.Command, root *os.Root, projectDir string, dryRun bool) error {
	const relPath = ".gitignore"
	absPath := filepath.Join(projectDir, relPath)

	var (
		existing []byte
		exists   bool
		mode     os.FileMode = 0o644
	)

	info, err := root.Lstat(relPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		exists = false
	case err != nil:
		return fmt.Errorf("inspect %q: %w", absPath, err)
	default:
		exists = true
		mode = info.Mode().Perm()
		existing, err = root.ReadFile(relPath)
		if err != nil {
			return fmt.Errorf("read existing %q: %w", absPath, err)
		}
	}

	updated, changed := agentinit.PlanGitignoreAppend(existing, exists)
	if !changed {
		cmd.Printf("Unchanged: %s\n", absPath)
		return nil
	}

	if !exists {
		if dryRun {
			cmd.Printf("Would create: %s\n", absPath)
			return nil
		}
		if err := root.WriteFile(relPath, updated, mode); err != nil {
			return fmt.Errorf("write %q: %w", absPath, err)
		}
		cmd.Printf("Created: %s\n", absPath)
		return nil
	}

	if dryRun {
		cmd.Printf("Would append: %s\n", absPath)
		return nil
	}
	// Append only the new suffix so a failed write cannot truncate existing rules.
	f, err := root.OpenFile(relPath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return fmt.Errorf("open %q: %w", absPath, err)
	}
	if _, err := f.Write(updated[len(existing):]); err != nil {
		_ = f.Close()
		return fmt.Errorf("update %q: %w", absPath, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("update %q: %w", absPath, err)
	}
	cmd.Printf("Appended: %s\n", absPath)
	return nil
}
