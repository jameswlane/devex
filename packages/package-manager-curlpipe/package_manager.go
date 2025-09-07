package main

import (
	"context"
	"fmt"
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
		p.EnsureAvailable()
		return p.handleRemove(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// handleInstall executes installation scripts from URLs
func (p *CurlpipePlugin) handleInstall(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no URLs specified for installation")
	}

	// Parse flags
	dryRun := false
	force := false
	scriptURLs := []string{}

	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "--force":
			force = true
		default:
			scriptURLs = append(scriptURLs, arg)
		}
	}

	if len(scriptURLs) == 0 {
		return fmt.Errorf("no URLs specified for installation")
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
			p.logger.Printf("[DRY RUN] Would execute: curl -fsSL %s | sh\n", scriptURL)
			continue
		}

		// Execute the installation script
		p.logger.Printf("Executing script from: %s\n", scriptURL)
		curlCmd := fmt.Sprintf("curl -fsSL %s | sh", scriptURL)
		if err := sdk.ExecCommandWithContext(ctx, true, "bash", "-c", curlCmd); err != nil {
			return fmt.Errorf("failed to execute script from '%s': %w", scriptURL, err)
		}

		p.logger.Success("Successfully executed script from %s", scriptURL)

		// Track installation (extract app name from URL)
		appName := p.extractAppNameFromURL(scriptURL)
		p.logger.Debug("Tracking installation: %s", appName)
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

	p.logger.Printf("Note: Curl pipe installations typically don't provide uninstall scripts\n")
	p.logger.Printf("The following applications cannot be automatically uninstalled: %s\n", strings.Join(args, ", "))
	p.logger.Printf("Manual removal may be required - check application documentation.\n")

	return nil
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
