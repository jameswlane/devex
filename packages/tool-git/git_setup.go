package main

import (
	"context"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// HandleSetup handles the setup protocol for Git configuration
// This is called by the DevEx CLI during the setup workflow
func (p *GitPlugin) HandleSetup(ctx context.Context, args []string) error {
	// Read setup input from stdin (JSON format)
	input, err := sdk.ReadSetupInput()
	if err != nil {
		return sdk.SendError("failed to read setup input", err)
	}

	// Send initial progress
	if err := sdk.SendProgress(10, "Configuring Git..."); err != nil {
		return err
	}

	// Extract name and email from parameters or config
	fullName, _ := input.GetParameterString("git_full_name")
	if fullName == "" {
		fullName, _ = input.GetConfigString("name")
	}

	email, _ := input.GetParameterString("git_email")
	if email == "" {
		email, _ = input.GetConfigString("email")
	}

	// Validate that we have the required information
	if fullName == "" || email == "" {
		return sdk.SendError("Git configuration requires both name and email", nil)
	}

	// Send progress update
	if err := sdk.SendProgress(30, "Setting user configuration..."); err != nil {
		return err
	}

	// Set user configuration
	if err := p.SetUserConfig(ctx, fullName, email); err != nil {
		return sdk.SendError("failed to set user config", err)
	}

	// Send progress update
	if err := sdk.SendProgress(60, "Applying sensible defaults..."); err != nil {
		return err
	}

	// Set sensible defaults
	if err := p.SetSensibleDefaults(ctx); err != nil {
		return sdk.SendError("failed to set defaults", err)
	}

	// Send success response
	data := map[string]interface{}{
		"user_name":  fullName,
		"user_email": email,
		"configured": true,
	}

	return sdk.SendSuccess("Git configured successfully", data)
}
