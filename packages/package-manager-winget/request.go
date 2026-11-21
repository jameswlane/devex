package main

import (
	"fmt"
	"regexp"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// request is a validated list of package identifiers and an optional scope.
type request struct {
	ids   []string
	scope string // "", "user" or "machine"
}

// parseRequest accepts package identifiers and at most one --scope=<user|machine>
// (or --scope <user|machine>). Every other option is rejected so callers cannot
// pass native winget flags through.
func parseRequest(args []string) (request, error) {
	var req request
	scopeSet := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			req.ids = append(req.ids, arg)
			continue
		}
		name, value, inline := strings.Cut(arg, "=")
		if name != "--scope" {
			return req, fmt.Errorf("unsupported option %q: only --scope=user or --scope=machine is accepted", arg)
		}
		if !inline {
			i++
			if i >= len(args) {
				return req, fmt.Errorf("option --scope requires user or machine")
			}
			value = args[i]
		}
		if value != "user" && value != "machine" {
			return req, fmt.Errorf("option --scope must be user or machine, not %q", value)
		}
		if scopeSet && value != req.scope {
			return req, fmt.Errorf("conflicting --scope values %q and %q", req.scope, value)
		}
		req.scope, scopeSet = value, true
	}
	if err := validateIDs(req.ids); err != nil {
		return req, err
	}
	return req, nil
}

func (r request) scopeArgs() []string {
	if r.scope == "" {
		return nil
	}
	return []string{"--scope", r.scope}
}

// privilege is system scope only for an explicit machine-scope change. Without
// a scope winget runs as the current user and asks Windows to elevate any
// installer that needs it, so a declined prompt is reported by explain.
func (r request) privilege() sdk.OperationScope {
	if r.scope == "machine" {
		return sdk.ScopeSystem
	}
	return sdk.ScopeUser
}

var (
	// Community repository identifiers follow the manifest schema: 2-8
	// dot-separated segments of up to 32 characters, such as Git.Git or
	// Microsoft.VisualStudioCode.Insiders.
	communityID = regexp.MustCompile(`^[^.\s\\/:*?"<>|\x00-\x1f]{1,32}(\.[^.\s\\/:*?"<>|\x00-\x1f]{1,32}){1,7}$`)
	// Microsoft Store identifiers, such as 9NBLGGH4NNS1 or XP89DCGQ3K6VLD.
	storeID = regexp.MustCompile(`^(9[0-9A-Z]{11}|XP[0-9A-Z]{12})$`)
)

func validateIDs(ids []string) error {
	if len(ids) == 0 {
		return fmt.Errorf("no package identifiers specified")
	}
	for _, id := range ids {
		if strings.HasPrefix(id, "-") || len(id) > 128 || (!communityID.MatchString(id) && !storeID.MatchString(id)) {
			return fmt.Errorf("invalid winget package identifier %q: use the exact Id column from winget search, such as Git.Git or 9NBLGGH4NNS1", id)
		}
	}
	return nil
}
