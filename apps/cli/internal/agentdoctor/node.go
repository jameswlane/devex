package agentdoctor

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// NodeCheck summarizes the Node.js version check for a Node-based agent.
type NodeCheck struct {
	Required string `json:"required"`
	Detected string `json:"detected,omitempty"`
	Path     string `json:"path,omitempty"`
	OK       bool   `json:"ok"`
}

type nodeProbeCache struct {
	probed   bool
	path     string
	detected string
	parsed   *semver.Version
}

func (c *nodeProbeCache) probe(ctx context.Context, probe Probe) {
	if c.probed {
		return
	}
	c.probed = true
	nodePath, ok := FindBinary(probe, "node")
	if !ok {
		return
	}
	c.path = nodePath
	cmdCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	out, err := probe.Run(cmdCtx, nodePath, "--version")
	if err != nil {
		return
	}
	raw := firstLineTrimmed(string(out), maxVersionLength)
	c.detected = raw
	ver, err := semver.NewVersion(strings.TrimPrefix(raw, "v"))
	if err == nil {
		c.parsed = ver
	}
}

func evaluateNodeCheck(ctx context.Context, probe Probe, cache *nodeProbeCache, minVersion string) NodeCheck {
	cache.probe(ctx, probe)
	nc := NodeCheck{
		Required: minVersion,
		Detected: cache.detected,
		Path:     cache.path,
	}
	req, err := semver.NewVersion(strings.TrimPrefix(minVersion, "v"))
	if err != nil || cache.parsed == nil {
		return nc
	}
	nc.OK = !cache.parsed.LessThan(req)
	return nc
}

func recommendedMiseNodeSpec(minVersion string) string {
	v, err := semver.NewVersion(strings.TrimPrefix(minVersion, "v"))
	if err != nil {
		return minVersion
	}
	if v.Minor() == 0 && v.Patch() == 0 {
		return fmt.Sprintf("%d", v.Major())
	}
	return v.String()
}
