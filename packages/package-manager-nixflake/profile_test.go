package main_test

import (
	"context"
	"os"
	"path/filepath"

	main "github.com/jameswlane/devex/packages/package-manager-nixflake"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("versioned Nix profile selectors", func() {
	DescribeTable("uses the native profile format",
		func(fixture, selector string) {
			recorder := sdktest.NewRecorder(GinkgoT(), "nix")
			data, err := os.ReadFile(filepath.Join("testdata", fixture))
			Expect(err).NotTo(HaveOccurred())
			recorder.Script("nix", []string{"profile", "list", "--json"}, sdktest.Response{Stdout: string(data)})
			Expect(main.NewNixflakePlugin().ExecuteContext(context.Background(), "remove", []string{"git"})).To(Succeed())
			Expect(recorder.Calls("nix")).To(Equal([]sdktest.Call{{Args: []string{"profile", "list", "--json"}}, {Args: []string{"profile", "remove", selector}}}))
		},
		Entry("Nix 2.18 indexed manifest", "profile-2.18.json", `legacyPackages\.x86_64-linux\.git`),
		Entry("Nix 2.24 named manifest", "profile-2.24.json", "git"),
	)
	It("resolves dotted short names before treating them as full legacy selectors", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "nix")
		recorder.Script("nix", []string{"profile", "list", "--json"}, sdktest.Response{Stdout: `{"version":2,"elements":[{"attrPath":"legacyPackages.x86_64-linux.python3.11"}]}`})
		Expect(main.NewNixflakePlugin().Execute("remove", []string{"python3.11"})).To(Succeed())
		Expect(operationCalls(recorder, "nix")).To(Equal([]sdktest.Call{{Args: []string{"profile", "remove", `legacyPackages\.x86_64-linux\.python3\.11`}}}))
	})
	DescribeTable("does not remove anything with unusable metadata", func(metadata string, message string) {
		recorder := sdktest.NewRecorder(GinkgoT(), "nix")
		recorder.Script("nix", []string{"profile", "list", "--json"}, sdktest.Response{Stdout: metadata})
		Expect(main.NewNixflakePlugin().Execute("remove", []string{"git"})).To(MatchError(ContainSubstring(message)))
		Expect(recorder.Calls("nix")).To(HaveLen(1))
	},
		Entry("malformed", "{", "read Nix profile"),
		Entry("future format", `{"version":4,"elements":{}}`, "unsupported"),
		Entry("absent", `{"version":2,"elements":[]}`, "not found"),
		Entry("ambiguous", `{"version":2,"elements":[{"attrPath":"a.git"},{"attrPath":"b.git"}]}`, "ambiguous"),
	)
})
