package sdk

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("macOS authorization adapter", func() {
	var (
		plan    ExecutionPlan
		streams OperationIO
		stdout  bytes.Buffer
		stderr  bytes.Buffer
	)

	BeforeEach(func() {
		plan = ExecutionPlan{
			ExecutionRequest: ExecutionRequest{
				Name: "/bin/sh",
				Args: []string{"-c", "printf '%s' 'argument with spaces'"},
			},
			Elevate: true,
		}
		stdout.Reset()
		stderr.Reset()
		streams = OperationIO{Stdout: &stdout, Stderr: &stderr}
	})

	It("passes argv separately and forwards structured successful output", func() {
		input := bytes.NewBufferString("input")
		streams.Stdin = input
		var gotArgs []string
		adapter := macOSAuthorizationAdapter{execute: func(_ context.Context, args []string, stdin io.Reader) ([]byte, []byte, error) {
			gotArgs = append([]string(nil), args...)
			Expect(stdin).To(BeIdenticalTo(input))
			return []byte(macOSAuthorizationResultPrefix + "0\ncommand output"), nil, nil
		}}

		Expect(adapter.Run(context.Background(), plan, streams)).To(Succeed())
		Expect(gotArgs).To(HaveLen(7))
		Expect(gotArgs[0]).To(Equal("-e"))
		Expect(gotArgs[1]).To(ContainSubstring("on error errorMessage number errorNumber"))
		Expect(gotArgs[2:]).To(Equal([]string{"--", plan.Name, plan.Args[0], plan.Args[1]}))
		Expect(stdout.String()).To(Equal("command output"))
		Expect(stderr.String()).To(BeEmpty())
	})

	It("preserves target status instead of classifying target stderr as denial", func() {
		adapter := macOSAuthorizationAdapter{execute: func(context.Context, []string, io.Reader) ([]byte, []byte, error) {
			return []byte(macOSAuthorizationResultPrefix + "2\nexample (-128)"), nil, nil
		}}

		err := adapter.Run(context.Background(), plan, streams)
		var authorization *AuthorizationError
		Expect(errors.As(err, &authorization)).To(BeFalse())
		var native NativeExitStatus
		Expect(errors.As(err, &native)).To(BeTrue())
		Expect(native.ExitCode()).To(Equal(2))
		Expect(ExitStatus(err)).To(Equal(2))
		Expect(stderr.String()).To(Equal("example (-128)"))
	})

	DescribeTable("classifies denial from the AppleScript error number",
		func(status int) {
			adapter := macOSAuthorizationAdapter{execute: func(context.Context, []string, io.Reader) ([]byte, []byte, error) {
				return []byte(macOSAuthorizationResultPrefix + strconv.Itoa(status) + "\nauthorization rejected"), nil, nil
			}}

			err := adapter.Run(context.Background(), plan, streams)
			var authorization *AuthorizationError
			Expect(errors.As(err, &authorization)).To(BeTrue())
			Expect(authorization.Kind).To(Equal(AuthorizationDenied))
			Expect(authorization.NativeExitCode).To(Equal(status))
			Expect(ExitStatus(err)).To(Equal(1))
		},
		Entry("user cancellation", -128),
		Entry("authorization denial", -60005),
		Entry("Authorization Services cancellation", -60006),
	)

	It("does not infer denial from osascript process stderr", func() {
		native := exec.CommandContext(context.Background(), "sh", "-c", "exit 1").Run()
		adapter := macOSAuthorizationAdapter{execute: func(context.Context, []string, io.Reader) ([]byte, []byte, error) {
			return nil, []byte("example (-128)"), native
		}}

		err := adapter.Run(context.Background(), plan, streams)
		var authorization *AuthorizationError
		Expect(errors.As(err, &authorization)).To(BeFalse())
		var exit *exec.ExitError
		Expect(errors.As(err, &exit)).To(BeTrue())
		Expect(ExitStatus(err)).To(Equal(1))
		Expect(stderr.String()).To(Equal("example (-128)"))
	})

	It("reports an osascript start failure as authorization required", func() {
		cause := errors.New("osascript unavailable")
		adapter := macOSAuthorizationAdapter{execute: func(context.Context, []string, io.Reader) ([]byte, []byte, error) {
			return nil, nil, cause
		}}

		err := adapter.Run(context.Background(), plan, streams)
		var authorization *AuthorizationError
		Expect(errors.As(err, &authorization)).To(BeTrue())
		Expect(authorization.Kind).To(Equal(AuthorizationRequired))
		Expect(errors.Is(err, cause)).To(BeTrue())
	})

	It("preserves context cancellation", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		adapter := macOSAuthorizationAdapter{execute: func(ctx context.Context, _ []string, _ io.Reader) ([]byte, []byte, error) {
			return nil, nil, ctx.Err()
		}}

		Expect(adapter.Run(ctx, plan, streams)).To(MatchError(context.Canceled))
	})

	It("rejects a missing structured result", func() {
		adapter := macOSAuthorizationAdapter{execute: func(context.Context, []string, io.Reader) ([]byte, []byte, error) {
			return []byte("unstructured output"), nil, nil
		}}

		Expect(adapter.Run(context.Background(), plan, streams)).To(MatchError(ContainSubstring("missing structured status")))
	})
})
