package skillpacks

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
	"time"
)

const requirementCheckTimeout = 5 * time.Second

// RequirementChecker injects PATH lookup and check execution for testing.
type RequirementChecker struct {
	LookPath func(file string) (string, error)
	RunCheck func(ctx context.Context, name string, args ...string) error
}

// DefaultRequirementChecker returns a RequirementChecker using the host PATH
// and bounded silent subprocess checks.
func DefaultRequirementChecker() RequirementChecker {
	return RequirementChecker{
		LookPath: exec.LookPath,
		RunCheck: func(ctx context.Context, name string, args ...string) error {
			cmdCtx, cancel := context.WithTimeout(ctx, requirementCheckTimeout)
			defer cancel()
			cmd := exec.CommandContext(cmdCtx, name, args...)
			cmd.WaitDelay = time.Second
			cmd.Stdout = io.Discard
			cmd.Stderr = io.Discard
			return cmd.Run()
		},
	}
}

func (c RequirementChecker) withDefaults() RequirementChecker {
	def := DefaultRequirementChecker()
	if c.LookPath == nil {
		c.LookPath = def.LookPath
	}
	if c.RunCheck == nil {
		c.RunCheck = def.RunCheck
	}
	return c
}

// MissingRequirement reports an unsatisfied pack requirement with an install or
// login hint. DevEx never installs requirements automatically.
type MissingRequirement struct {
	Command string
	Reason  string
	Hint    string
}

// CheckRequirements evaluates pack requirements whose skill filter intersects
// selectedSkills (an empty skill filter applies to the whole pack).
func CheckRequirements(ctx context.Context, checker RequirementChecker, reqs []Requirement, selectedSkills []string) []MissingRequirement {
	checker = checker.withDefaults()
	var missing []MissingRequirement
	for _, req := range reqs {
		if len(req.Skills) > 0 && !intersects(req.Skills, selectedSkills) {
			continue
		}
		if _, err := checker.LookPath(req.Command); err != nil {
			missing = append(missing, MissingRequirement{
				Command: req.Command,
				Reason:  "not found on PATH",
				Hint:    req.Hint,
			})
			continue
		}
		if len(req.Check) > 0 {
			if err := checker.RunCheck(ctx, req.Check[0], req.Check[1:]...); err != nil {
				missing = append(missing, MissingRequirement{
					Command: req.Command,
					Reason:  fmt.Sprintf("%q check failed", strings.Join(req.Check, " ")),
					Hint:    req.Hint,
				})
			}
		}
	}
	return missing
}

func intersects(a, b []string) bool {
	for _, item := range a {
		if slices.Contains(b, item) {
			return true
		}
	}
	return false
}
