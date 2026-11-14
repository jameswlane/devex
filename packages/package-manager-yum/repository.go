package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

func (p *YumPlugin) handleAddRepository(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("add-repository requires exactly one argument")
	}
	value := args[0]
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" ||
		!strings.HasSuffix(u.Path, ".repo") || strings.ContainsAny(value, "\r\n\t ") {
		return fmt.Errorf("invalid repository source: expected HTTPS .repo URL")
	}
	if _, err := p.lookPath("yum-config-manager"); err != nil {
		return fmt.Errorf("yum-config-manager not found on PATH; install yum-utils and retry")
	}
	return sdk.ExecCommandWithContext(ctx, true, "yum-config-manager", "--add-repo", value)
}
