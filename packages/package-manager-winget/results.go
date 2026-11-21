package main

import (
	"errors"
	"fmt"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// winget exits with an HRESULT. These values come from winget-cli's
// src/AppInstallerSharedLib/Public/AppInstallerErrors.h.
const (
	hrNoApplicableInstaller        uint32 = 0x8A150010
	hrNoApplicationsFound          uint32 = 0x8A150014
	hrCommandRequiresAdmin         uint32 = 0x8A150019
	hrUpdateNotApplicable          uint32 = 0x8A15002B
	hrInstallerProhibitsElevation  uint32 = 0x8A150056
	hrPackageAlreadyInstalled      uint32 = 0x8A150061
	hrAdminContextActionProhibited uint32 = 0x8A15007D
	hrInstallCancelledByUser       uint32 = 0x8A15010C
	hrInstallAlreadyInstalled      uint32 = 0x8A15010D
	// HRESULT_FROM_WIN32(ERROR_CANCELLED): the UAC prompt winget raised for
	// an installer that needs elevation was declined.
	hrElevationCancelled uint32 = 0x800704C7
)

// fakeExitCodes lets specs stand in for HRESULTs: recorder fakes are shell
// scripts and can only exit 0-255. It is nil outside tests.
var fakeExitCodes map[int]uint32

// hresult returns winget's exit status as the unsigned HRESULT it reports.
func hresult(err error) (uint32, bool) {
	var exit sdk.NativeExitStatus
	if err == nil || !errors.As(err, &exit) {
		return 0, false
	}
	var auth *sdk.AuthorizationError
	if errors.As(err, &auth) {
		return 0, false
	}
	code := exit.ExitCode()
	if mapped, ok := fakeExitCodes[code]; ok {
		return mapped, true
	}
	// Windows reports the status as an unsigned 32-bit value; keep the bits.
	return uint32(int32(code)), true //nolint:gosec // HRESULTs are 32-bit by definition
}

// isAlreadyDone reports a winget result that means the change is already in
// place: an installed package for install, a current one for upgrade.
func isAlreadyDone(command string, code uint32) bool {
	switch command {
	case "install":
		return code == hrPackageAlreadyInstalled || code == hrInstallAlreadyInstalled
	case "upgrade":
		return code == hrUpdateNotApplicable
	}
	return false
}

func alreadyDoneMessage(command string) string {
	if command == "upgrade" {
		return "already up to date"
	}
	return "already installed; use upgrade to update it"
}

// explain adds actionable guidance to a failed change while keeping winget's
// exit status (and the SDK's authorization errors) for the caller.
func explain(command, id, scope string, err error) error {
	code, ok := hresult(err)
	if !ok {
		return fmt.Errorf("winget %s %s failed: %w", nativeVerb[command], id, err)
	}
	switch code {
	case hrElevationCancelled:
		return sdk.NewAuthorizationDeniedError(fmt.Sprintf(
			"Windows elevation was declined for %s; approve the UAC prompt, or run devex from a terminal opened with Run as administrator and use --scope=machine", id),
			sdk.ExitStatus(err), err)
	case hrInstallCancelledByUser:
		return fmt.Errorf("the %s installer for %s was cancelled, possibly by a declined UAC prompt; approve the prompt or run from an elevated terminal with --scope=machine: %w", nativeVerb[command], id, err)
	case hrCommandRequiresAdmin:
		return fmt.Errorf("winget %s %s requires administrator rights; run devex from a terminal opened with Run as administrator and use --scope=machine: %w", nativeVerb[command], id, err)
	case hrInstallerProhibitsElevation, hrAdminContextActionProhibited:
		return fmt.Errorf("winget %s %s cannot run from an elevated terminal; run devex from a regular terminal without --scope=machine: %w", nativeVerb[command], id, err)
	case hrNoApplicableInstaller:
		if scope != "" {
			return fmt.Errorf("%s has no installer for %s scope on this system; use the other scope or omit --scope: %w", id, scope, err)
		}
		return fmt.Errorf("%s has no installer for this system's architecture, scope or Windows version: %w", id, err)
	case hrNoApplicationsFound:
		if command == "install" {
			return fmt.Errorf("no winget package has the identifier %s; check it with winget search: %w", id, err)
		}
		return fmt.Errorf("%s is not installed (winget found no installed package with this identifier): %w", id, err)
	}
	return fmt.Errorf("winget %s %s failed with 0x%08X: %w", nativeVerb[command], id, code, err)
}
