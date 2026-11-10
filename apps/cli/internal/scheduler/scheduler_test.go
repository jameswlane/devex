package scheduler_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/scheduler"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestScheduler(t *testing.T) { RegisterFailHandler(Fail); RunSpecs(t, "Installation scheduler") }

var _ = Describe("Scheduling", func() {
	It("overlaps independent resources and bounds workers without starving unlocked work", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		started := make(chan string, 3)
		release := make(chan struct{})
		done := make(chan error, 1)
		run := func(id string) func(context.Context) error {
			return func(ctx context.Context) error {
				started <- id
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
		go func() {
			done <- scheduler.Run(ctx, []scheduler.Task{{ID: "apt", Resource: "apt", Run: run("apt")}, {ID: "dpkg", Resource: "apt", Run: run("dpkg")}, {ID: "mise", Resource: "mise", Run: run("mise")}}, 2)
		}()
		var first, second string
		Eventually(started).Should(Receive(&first))
		Eventually(started).Should(Receive(&second))
		Expect([]string{first, second}).To(ConsistOf("apt", "mise"))
		Expect(started).NotTo(Receive())
		close(release)
		Eventually(done).Should(Receive(Succeed()))
		Expect(started).To(Receive(Equal("dpkg")))
	})
	It("orders dependencies even when listed after consumers and respects barriers", func() {
		var mu sync.Mutex
		var order []string
		record := func(id string) func(context.Context) error {
			return func(context.Context) error { mu.Lock(); defer mu.Unlock(); order = append(order, id); return nil }
		}
		Expect(scheduler.Run(context.Background(), []scheduler.Task{{ID: "consumer", DependsOn: []string{"base"}, Exclusive: true, Run: record("consumer")}, {ID: "base", Run: record("base")}, {ID: "last", Run: record("last")}}, 3)).To(Succeed())
		Expect(order).To(Equal([]string{"base", "consumer", "last"}))
	})
	It("serializes aliases sharing native package databases", func() {
		for _, pair := range [][2]string{{"apt", "dpkg"}, {"dnf", "rpm"}, {"yum", "dnf"}} {
			Expect(scheduler.ManagerResource(pair[0])).To(Equal(scheduler.ManagerResource("package-manager-" + pair[1])))
		}
		for _, method := range []string{"pacman", "zypper", "snap", "flatpak"} {
			Expect(scheduler.ManagerResource(method)).To(Equal(method))
		}
	})
	It("aggregates independent failures and skips failed dependents", func() {
		one, two := errors.New("one"), errors.New("two")
		err := scheduler.Run(context.Background(), []scheduler.Task{{ID: "one", Run: func(context.Context) error { return one }}, {ID: "two", Run: func(context.Context) error { return two }}, {ID: "skip", DependsOn: []string{"one"}, Run: func(context.Context) error { Fail("dependent ran"); return nil }}}, 2)
		Expect(errors.Is(err, one)).To(BeTrue())
		Expect(errors.Is(err, two)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("dependency failed"))
	})
	It("cancels active work and never starts pending work", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		entered := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			done <- scheduler.Run(ctx, []scheduler.Task{{ID: "active", Run: func(ctx context.Context) error { close(entered); <-ctx.Done(); return ctx.Err() }}, {ID: "pending", Run: func(context.Context) error { Fail("pending ran"); return nil }}}, 1)
		}()
		Eventually(entered).Should(BeClosed())
		cancel()
		var err error
		Eventually(done).Should(Receive(&err))
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
	})
	It("validates graphs and limits before executing", func() {
		run := func(context.Context) error { Fail("invalid graph executed"); return nil }
		for _, tasks := range [][]scheduler.Task{{{ID: "a", DependsOn: []string{"missing"}, Run: run}}, {{ID: "a", DependsOn: []string{"b"}, Run: run}, {ID: "b", DependsOn: []string{"a"}, Run: run}}, {{ID: "a", Run: run}, {ID: "a", Run: run}}} {
			Expect(scheduler.Run(context.Background(), tasks, 2)).To(HaveOccurred())
		}
		Expect(scheduler.Run(context.Background(), nil, 0)).To(MatchError(ContainSubstring("at least 1")))
	})
	It("stops scheduling on a fatal failure and recovers panics", func() {
		err := scheduler.Run(context.Background(), []scheduler.Task{{ID: "bad", StopOnError: true, Run: func(context.Context) error { panic("broken") }}, {ID: "pending", Run: func(context.Context) error { Fail("pending ran"); return nil }}}, 1)
		Expect(err).To(MatchError(ContainSubstring("executor panic: broken")))
	})
})
