package main

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handlePreview downloads and displays a script before execution
func (p *CurlpipePlugin) handlePreview(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no URL specified")
	}

	scriptURL := args[0]

	// Validate URL format
	if err := p.ValidateScriptURL(scriptURL); err != nil {
		return fmt.Errorf("invalid URL '%s': %w", scriptURL, err)
	}

	// Validate trusted domain
	if err := p.validateTrustedDomain(scriptURL); err != nil {
		p.logger.Printf("⚠️  Warning: Untrusted domain - %v\n", err)
		if !p.confirmUntrustedPreview() {
			return fmt.Errorf("preview cancelled by user")
		}
	}

	p.logger.Printf("Downloading and previewing script from: %s\n", scriptURL)
	p.logger.Printf("%s\n", strings.Repeat("=", 60))

	// Download and display the script
	if err := p.downloadScript(ctx, scriptURL); err != nil {
		return fmt.Errorf("failed to download script from '%s': %w", scriptURL, err)
	}

	p.logger.Printf("%s\n", strings.Repeat("=", 60))
	p.logger.Printf("End of script preview from: %s\n", scriptURL)
	return nil
}

// downloadScript downloads a script from the given URL
func (p *CurlpipePlugin) downloadScript(ctx context.Context, scriptURL string) error {
	// Use curl to download the script content
	return sdk.ExecCommandWithContext(ctx, false, "curl", "-fsSL", scriptURL)
}

// DownloadScriptToString downloads a script and returns its content as a string
func (p *CurlpipePlugin) DownloadScriptToString(ctx context.Context, scriptURL string) (string, error) {
	output, err := sdk.ExecCommandOutputWithContext(ctx, "curl", "-fsSL", scriptURL)
	if err != nil {
		return "", fmt.Errorf("failed to download script: %w", err)
	}

	return p.SanitizeScriptContent(output), nil
}

// confirmUntrustedPreview prompts user for confirmation to preview untrusted scripts
func (p *CurlpipePlugin) confirmUntrustedPreview() bool {
	p.logger.Printf("Continue anyway? (y/N): ")
	var response string
	_, _ = fmt.Scanln(&response)
	return strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
}

// SanitizeScriptContent sanitizes script content for safe display
func (p *CurlpipePlugin) SanitizeScriptContent(content string) string {
	// Remove null bytes
	content = strings.ReplaceAll(content, "\x00", "")

	// Limit content length for preview (max 50KB)
	const maxPreviewLength = 50 * 1024
	if len(content) > maxPreviewLength {
		content = content[:maxPreviewLength] + "\n\n...[Content truncated for safety - script is longer than 50KB]"
	}

	return content
}

// ValidateScriptContent performs basic validation on script content
func (p *CurlpipePlugin) ValidateScriptContent(content string) error {
	if content == "" {
		return fmt.Errorf("script content is empty")
	}

	// Check for null bytes
	if strings.Contains(content, "\x00") {
		return fmt.Errorf("script contains null bytes")
	}

	// Check for extremely long lines that might indicate binary content
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if len(line) > 10000 {
			return fmt.Errorf("script contains suspiciously long lines - may be binary content")
		}
	}

	return nil
}

// ExecuteScriptFromURL downloads and executes a script from URL with validation
func (p *CurlpipePlugin) ExecuteScriptFromURL(ctx context.Context, scriptURL string) error {
	p.logger.Printf("Executing script from: %s\n", scriptURL)

	// First download and validate the script content
	scriptContent, err := p.DownloadScriptToString(ctx, scriptURL)
	if err != nil {
		return fmt.Errorf("failed to download script for validation: %w", err)
	}

	// Validate script content before execution
	if err := p.ValidateScriptContent(scriptContent); err != nil {
		return fmt.Errorf("script validation failed: %w", err)
	}

	// Perform additional runtime security checks
	if err := p.ValidateScriptSecurity(scriptContent); err != nil {
		return fmt.Errorf("script security validation failed: %w", err)
	}

	p.logger.Printf("Script content validated successfully, proceeding with execution\n")

	// Execute the validated script using bash with the script content
	if err := sdk.ExecCommandWithContext(ctx, true, "bash", "-c", scriptContent); err != nil {
		return fmt.Errorf("failed to execute script: %w", err)
	}

	return nil
}

// CheckCurlAvailability checks if curl is available on the system
func (p *CurlpipePlugin) CheckCurlAvailability() error {
	if !sdk.CommandExists("curl") {
		return fmt.Errorf("curl is not installed or not available in PATH")
	}
	return nil
}
