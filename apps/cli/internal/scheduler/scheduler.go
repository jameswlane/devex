// Package scheduler runs dependency graphs with bounded, resource-safe concurrency.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Task dependencies refer to IDs in this run. Exclusive tasks form barriers.
type Task struct {
	ID            string
	DependsOn     []string
	Resource      string
	StopOnError   bool
	IgnoreFailure bool
	Exclusive     bool
	Run           func(context.Context) error
}

// ManagerResource maps aliases that share a native database to the same lock.
func ManagerResource(manager string) string {
	manager = strings.TrimPrefix(manager, "package-manager-")
	switch manager {
	case "apt", "dpkg":
		return "apt/dpkg"
	case "dnf", "rpm", "yum":
		return "dnf/rpm"
	default:
		return manager
	}
}

// Run drains running tasks on cancellation or failure. Failed dependencies are
// skipped, unrelated tasks continue, and all failures are returned in dependency order.
// Resource locks cover this invocation; they do not replace OS package locks.
func Run(ctx context.Context, tasks []Task, limit int) error {
	if limit < 1 {
		return fmt.Errorf("install-concurrency must be at least 1")
	}
	ids := make(map[string]int, len(tasks))
	for i, t := range tasks {
		if t.ID == "" || t.Run == nil {
			return fmt.Errorf("task %d requires an id and executor", i)
		}
		if _, ok := ids[t.ID]; ok {
			return fmt.Errorf("duplicate task id %q", t.ID)
		}
		ids[t.ID] = i
	}
	for _, t := range tasks {
		for _, dep := range t.DependsOn {
			if _, ok := ids[dep]; !ok {
				return fmt.Errorf("task %s: unknown dependency %s", t.ID, dep)
			}
		}
	}
	// Validate the complete graph before any side effects.
	ordered := make([]Task, 0, len(tasks))
	visiting, visited := make([]bool, len(tasks)), make([]bool, len(tasks))
	var visit func(int) error
	visit = func(i int) error {
		if visiting[i] {
			return fmt.Errorf("dependency cycle at %s", tasks[i].ID)
		}
		if visited[i] {
			return nil
		}
		visiting[i] = true
		for _, dep := range tasks[i].DependsOn {
			if err := visit(ids[dep]); err != nil {
				return err
			}
		}
		visiting[i], visited[i] = false, true
		ordered = append(ordered, tasks[i])
		return nil
	}
	for i := range tasks {
		if err := visit(i); err != nil {
			return err
		}
	}
	tasks = ordered
	for i, t := range tasks {
		ids[t.ID] = i
	}
	type completion struct {
		index int
		err   error
	}
	done := make(chan completion, len(tasks))
	status := make([]int, len(tasks)) // pending, running, succeeded, failed
	failures := make([]error, len(tasks), len(tasks)+1)
	locks := map[string]bool{}
	stopped := false
	active, remaining := 0, len(tasks)
	for remaining > 0 {
		progress := false
		for i, t := range tasks {
			if status[i] != 0 {
				continue
			}
			ready, failed := true, false
			for _, dep := range t.DependsOn {
				s := status[ids[dep]]
				ready = ready && s >= 2
				failed = failed || s == 3
			}
			if ready && failed {
				status[i] = 3
				failures[i] = fmt.Errorf("%s: dependency failed", t.ID)
				remaining--
				progress = true
				continue
			}
			if stopped || ctx.Err() != nil || !ready || active >= limit || locks[t.Resource] && t.Resource != "" {
				continue
			}
			// Barriers preserve implicit ordering for shell/file/service operations and hooks.
			blocked := false
			for j, other := range tasks {
				if j < i && status[j] < 2 && (t.Exclusive || other.Exclusive) {
					blocked = true
				}
			}
			if blocked || t.Exclusive && active > 0 {
				continue
			}
			for j, other := range tasks {
				if status[j] == 1 && other.Exclusive {
					blocked = true
				}
			}
			if blocked {
				continue
			}
			status[i] = 1
			active++
			locks[t.Resource] = true
			progress = true
			go func() {
				var err error
				defer func() {
					if p := recover(); p != nil {
						err = fmt.Errorf("%s: executor panic: %v", t.ID, p)
					}
					done <- completion{i, err}
				}()
				err = t.Run(ctx)
			}()
		}
		if active > 0 {
			c := <-done
			active--
			remaining--
			progress = true
			delete(locks, tasks[c.index].Resource)
			status[c.index] = 2
			if c.err != nil {
				status[c.index] = 3
				if !tasks[c.index].IgnoreFailure {
					failures[c.index] = c.err
				}
				if tasks[c.index].StopOnError {
					stopped = true
				}
			}
		}
		if !progress {
			break
		}
	}
	return errors.Join(append(failures, ctx.Err())...)
}
