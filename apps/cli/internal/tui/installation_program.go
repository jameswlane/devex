package tui

import (
	"context"
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// installationProgram allows the lifecycle to be exercised without a terminal.
type installationProgram interface {
	Run() (tea.Model, error)
	Send(tea.Msg)
	Kill()
}

// runInstallationProgram joins the worker before cleaning up model channels.
// A user quit before the worker finishes is cancellation, even if that worker
// ignores cancellation and returns nil. TUI and worker failures are both kept.
func runInstallationProgram(ctx context.Context, cancel context.CancelFunc, model *Model, program installationProgram, install func() error) (result error) {
	outcome := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		var err error
		defer func() {
			if recovered := recover(); recovered != nil {
				err = errors.Join(err, fmt.Errorf("installation panic: %v", recovered))
			}
			outcome <- err
			program.Send(tea.Quit())
		}()
		err = install()
	}()
	defer func() {
		if recovered := recover(); recovered != nil {
			result = errors.Join(result, fmt.Errorf("installation TUI panic: %v", recovered))
		}
		cancel()
		program.Kill()
		<-done
		// On a Run panic the normal path has not yet consumed the worker result.
		select {
		case err := <-outcome:
			result = errors.Join(result, err)
		default:
		}
		model.CleanupChannels()
	}()
	_, runErr := program.Run()
	select {
	case err := <-outcome:
		return errors.Join(runErr, ctx.Err(), err)
	default:
		cancel()
		return errors.Join(runErr, context.Canceled, <-outcome)
	}
}
