package main

import (
	"fmt"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handlePreview downloads and displays a script before execution
func (p *CurlpipePlugin) handlePreview(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no URL specified")
	}
	
	scriptURL := args[0]
	
	// Validate URL format
	if err := p.validateScriptURL(scriptURL); err != nil {
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
	if err := p.downloadScript(scriptURL); err != nil {
		return fmt.Errorf("failed to download script from '%s': %w", scriptURL, err)
	}
	
	p.logger.Printf("%s\n", strings.Repeat("=", 60))
	p.logger.Printf("End of script preview from: %s\n", scriptURL)
	return nil
}

// downloadScript downloads a script from the given URL
func (p *CurlpipePlugin) downloadScript(scriptURL string) error {
	// Use curl to download the script content
	return sdk.ExecCommand(false, "curl", "-fsSL", scriptURL)
}

// downloadScriptToString downloads a script and returns its content as a string
func (p *CurlpipePlugin) downloadScriptToString(scriptURL string) (string, error) {
	output, err := sdk.ExecCommandOutput("curl", "-fsSL", scriptURL)
	if err != nil {
		return "", fmt.Errorf("failed to download script: %w", err)
	}
	
	return p.sanitizeScriptContent(output), nil
}

// confirmUntrustedPreview prompts user for confirmation to preview untrusted scripts
func (p *CurlpipePlugin) confirmUntrustedPreview() bool {
	p.logger.Printf("Continue anyway? (y/N): ")
	var response string
	fmt.Scanln(&response)
	return strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
}

// sanitizeScriptContent sanitizes script content for safe display
func (p *CurlpipePlugin) sanitizeScriptContent(content string) string {
	// Remove null bytes
	content = strings.ReplaceAll(content, "\x00", "")
	
	// Limit content length for preview (max 50KB)
	const maxPreviewLength = 50 * 1024
	if len(content) > maxPreviewLength {
		content = content[:maxPreviewLength] + "\n\n...[Content truncated for safety - script is longer than 50KB]"
	}
	
	return content
}

// validateScriptContent performs basic validation on script content
func (p *CurlpipePlugin) validateScriptContent(content string) error {
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

// executeScriptFromURL downloads and executes a script from URL
func (p *CurlpipePlugin) executeScriptFromURL(scriptURL string) error {
	p.logger.Printf("Executing script from: %s\n", scriptURL)
	
	// Use bash to execute the curl pipe
	curlCmd := fmt.Sprintf("curl -fsSL %s | sh", scriptURL)
	if err := sdk.ExecCommand(true, "bash", "-c", curlCmd); err != nil {
		return fmt.Errorf("failed to execute script: %w", err)
	}
	
	return nil
}

// checkCurlAvailability checks if curl is available on the system
func (p *CurlpipePlugin) checkCurlAvailability() error {
	if !sdk.CommandExists("curl") {
		return fmt.Errorf("curl is not installed or not available in PATH")
	}
	return nil
}