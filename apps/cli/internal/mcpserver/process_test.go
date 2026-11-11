package mcpserver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/mcpserver"
)

var _ = Describe("devex mcp serve process", Label("process"), func() {
	var binary string

	BeforeEach(func(ctx SpecContext) {
		goBinary, err := exec.LookPath("go")
		if err != nil {
			Skip("the go toolchain is required to build the devex binary")
		}
		binary = filepath.Join(GinkgoT().TempDir(), "devex")
		build := exec.CommandContext(ctx, goBinary, "build", "-o", binary, "../../cmd")
		output, err := build.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
	}, NodeTimeout(5*time.Minute))

	// serve runs the read-only server with the given PATH under an empty home
	// and checks that it speaks MCP, serves every tool and writes nothing.
	serve := func(ctx SpecContext, path string) {
		home := GinkgoT().TempDir()
		var stderr bytes.Buffer
		command := exec.CommandContext(ctx, binary, "mcp", "serve")
		command.Env = []string{
			"HOME=" + home,
			"PATH=" + path,
			// An unreachable registry proves the server never contacts it.
			"DEVEX_PLUGIN_REGISTRY_URL=http://127.0.0.1:9",
		}
		command.Stderr = &stderr

		client := mcp.NewClient(&mcp.Implementation{Name: "process-test", Version: "0"}, nil)
		session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
		Expect(err).NotTo(HaveOccurred(), stderr.String())

		tools, err := session.ListTools(ctx, nil)
		Expect(err).NotTo(HaveOccurred())
		names := make([]string, 0, len(tools.Tools))
		for _, tool := range tools.Tools {
			names = append(names, tool.Name)
		}
		Expect(names).To(ConsistOf(mcpserver.ReadOnlyTools))

		for _, name := range mcpserver.ReadOnlyTools {
			arguments := map[string]any{}
			if name == "plan_install" {
				arguments["apps"] = []string{"git"}
			}
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
			Expect(err).NotTo(HaveOccurred(), name)
			Expect(result.IsError).To(BeFalse(), "%s: %v", name, result.Content)
		}
		_, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "install_apps", Arguments: map[string]any{"apps": []string{"git"}}})
		Expect(err).To(MatchError(ContainSubstring("unknown tool")))
		resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: mcpserver.EffectiveConfigURI})
		Expect(err).NotTo(HaveOccurred())
		Expect(resource.Contents[0].Text).To(ContainSubstring(`"applications"`))

		Expect(session.Close()).To(Succeed())

		var created []string
		Expect(filepath.Walk(home, func(path string, _ os.FileInfo, err error) error {
			if err == nil && path != home {
				created = append(created, path)
			}
			return err
		})).To(Succeed())
		Expect(created).To(BeEmpty(), "mcp serve created files under HOME")
		Expect(stderr.String()).NotTo(ContainSubstring("panic"))
	}

	It("speaks MCP on stdout, keeps diagnostics on stderr and writes nothing under an empty home", func(ctx SpecContext) {
		serve(ctx, os.Getenv("PATH"))
	}, SpecTimeout(2*time.Minute))

	It("starts without installer tools such as curl, git or unzip on PATH", func(ctx SpecContext) {
		Expect(filepath.IsAbs(binary)).To(BeTrue())
		serve(ctx, GinkgoT().TempDir())
	}, SpecTimeout(2*time.Minute))

	It("serves the mutating tools over the reserved stdout with --allow-mutations", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		var stderr bytes.Buffer
		command := exec.CommandContext(ctx, binary, "mcp", "serve", "--allow-mutations")
		command.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "DEVEX_PLUGIN_REGISTRY_URL=http://127.0.0.1:9", "DEVEX_OFFLINE=true"}
		command.Stderr = &stderr

		client := mcp.NewClient(&mcp.Implementation{Name: "process-test", Version: "0"}, nil)
		session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
		Expect(err).NotTo(HaveOccurred(), stderr.String())
		defer func() { _ = session.Close() }()

		tools, err := session.ListTools(ctx, nil)
		Expect(err).NotTo(HaveOccurred())
		names := make([]string, 0, len(tools.Tools))
		for _, tool := range tools.Tools {
			names = append(names, tool.Name)
		}
		Expect(names).To(ContainElements(mcpserver.MutatingTools))

		// Preparing only plans; nothing is installed.
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "prepare_install", Arguments: map[string]any{"apps": []string{"git"}}})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.IsError).To(BeFalse(), "%v %s", result.Content, stderr.String())
		Expect(result.StructuredContent).To(HaveKey("plan_id"))
	}, SpecTimeout(2*time.Minute))
})
