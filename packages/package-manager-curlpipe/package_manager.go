package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// CurlpipePlugin implements the Curl Pipe package manager
type CurlpipePlugin struct {
	*sdk.PackageManagerPlugin
	logger sdk.Logger
}

// Execute handles command execution
func (p *CurlpipePlugin) Execute(command string, args []string) error {
	ctx := context.Background()
	switch command {
	case "validate-url":
		return p.handleValidateURL(ctx, args)
	case "list-trusted":
		return p.handleListTrusted(ctx, args)
	case "preview":
		return p.handlePreview(ctx, args)
	case "install":
		p.EnsureAvailable()
		return p.handleInstall(ctx, args)
	case "remove":
		return p.handleRemove(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// handleInstall executes installation scripts from URLs
func (p *CurlpipePlugin) handleInstall(ctx context.Context, args []string) error {
	args, verification, parseErr := sdk.ParseVerificationArgs(args)
	if parseErr != nil {
		return parseErr
	}

	if len(args) == 0 {
		return fmt.Errorf("no URLs specified for installation")
	}

	// Parse flags
	dryRun := false
	force := false
	userScope := false
	var scriptURLs []string

	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "--force":
			force = true
		case "--user":
			userScope = true
		default:
			scriptURLs = append(scriptURLs, arg)
		}
	}

	if len(scriptURLs) == 0 {
		return fmt.Errorf("no URLs specified for installation")
	}

	if len(verification.Arguments()) > 0 && len(scriptURLs) != 1 {
		return fmt.Errorf("verification metadata requires exactly one artifact")
	}
	// Validate all URLs first
	for _, scriptURL := range scriptURLs {
		if err := p.ValidateScriptURL(scriptURL); err != nil {
			return fmt.Errorf("invalid URL '%s': %w", scriptURL, err)
		}

		// Validate trusted domain unless forced
		if !force {
			if err := p.validateTrustedDomain(scriptURL); err != nil {
				return fmt.Errorf("untrusted domain for URL '%s': %w (use --force to skip validation)", scriptURL, err)
			}
		}
	}

	p.logger.Printf("Executing installation scripts: %s\n", strings.Join(scriptURLs, ", "))

	for _, scriptURL := range scriptURLs {
		if dryRun {
			p.logger.Printf("[DRY RUN] Would execute script from: %s\n", scriptURL)
			continue
		}

		// Execute the installation script securely without shell interpolation
		p.logger.Printf("Executing script from: %s\n", scriptURL)

		tempFile, err := os.CreateTemp("", "curlpipe-install-*.sh")
		if err != nil {
			return fmt.Errorf("failed to create temp file for script: %w", err)
		}
		defer func() { _ = os.Remove(tempFile.Name()) }()
		_ = tempFile.Close()

		// Download to temp file with curl ... -- <url> as separate argv
		if verification.CachedPath != "" {
			if err := sdk.CopyCachedArtifact(ctx, verification.CachedPath, tempFile.Name()); err != nil {
				return err
			}
		} else if err := sdk.ExecCommandWithContext(ctx, false, "curl", "-fsSL", "-o", tempFile.Name(), "--", scriptURL); err != nil {
			return fmt.Errorf("failed to download script from '%s': %w", scriptURL, err)
		}

		verified, err := sdk.VerifyArtifact(ctx, tempFile.Name(), verification)
		if err != nil {
			return fmt.Errorf("download integrity verification failed: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Download verification: checksum=%t signature=%t (false means skipped)\n", verified.ChecksumVerified, verified.SignatureVerified)
		// Run the file with bash as a separate argv
		shell, shellArgs := scriptCommand(tempFile.Name())
		if userScope {
			// When DevEx is elevated the script runs as the invoking user, who
			// must be able to read the root-owned temp file. Install scripts are
			// public downloads, so read access leaks nothing.
			// #nosec G302 -- see above
			if err := os.Chmod(tempFile.Name(), 0o644); err != nil {
				return fmt.Errorf("failed to make script readable for the invoking user: %w", err)
			}
			if err := sdk.ExecOperationWithContext(ctx, sdk.OperationPrivilege{Scope: sdk.ScopeUser}, shell, shellArgs...); err != nil {
				return fmt.Errorf("failed to execute script from '%s': %w", scriptURL, err)
			}
		} else {
			if err := sdk.ExecCommandWithContext(ctx, true, shell, shellArgs...); err != nil {
				return fmt.Errorf("failed to execute script from '%s': %w", scriptURL, err)
			}
		}

		p.logger.Success("Successfully executed script from %s", scriptURL)

		// Track installation (extract app name from URL)
		appName := p.extractAppNameFromURL(scriptURL)
		p.logger.Debug(fmt.Sprintf("Tracking installation: %s", appName))
	}

	return nil
}

// handleRemove handles removal requests (limited support for curl pipe)
func (p *CurlpipePlugin) handleRemove(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no applications specified")
	}

	// Validate application names
	for _, app := range args {
		if err := p.validateAppName(app); err != nil {
			return fmt.Errorf("invalid application name '%s': %w", app, err)
		}
	}

	return &sdk.ManualRemovalRequiredError{Applications: args}
}

// handleValidateURL validates URLs for installation
func (p *CurlpipePlugin) handleValidateURL(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no URL specified")
	}

	for _, scriptURL := range args {
		if err := p.ValidateScriptURL(scriptURL); err != nil {
			p.logger.Printf("❌ %s: Invalid URL format - %v\n", scriptURL, err)
			continue
		}

		if err := p.validateTrustedDomain(scriptURL); err != nil {
			p.logger.Printf("⚠️ %s: Untrusted domain - %v\n", scriptURL, err)
		} else {
			p.logger.Printf("✅ %s: Trusted domain\n", scriptURL)
		}
	}

	return nil
}

// handleListTrusted lists all trusted domains
func (p *CurlpipePlugin) handleListTrusted(ctx context.Context, args []string) error {
	p.logger.Printf("Trusted domains for curl pipe installations:\n")
	trustedDomains := p.GetTrustedDomains()
	for i, domain := range trustedDomains {
		p.logger.Printf("%d. %s\n", i+1, domain)
	}
	p.logger.Printf("\nNote: Use --force flag to bypass domain validation (use with extreme caution)\n")
	return nil
}

// scriptCommand runs an install script with bash. Unattended runs detach
// the script from the controlling terminal, so installers that prompt
// through /dev/tty (Codex asks whether to launch) cannot block.
func scriptCommand(script string) (string, []string) {
	if sdk.Unattended() {
		if _, err := exec.LookPath("setsid"); err == nil {
			return "setsid", []string{"-w", "bash", script}
		}
	}
	return "bash", []string{script}
}
