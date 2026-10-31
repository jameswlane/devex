package selfupdate

import (
	"context"
	"os"
	"os/exec"
	"time"
)

const notificationHelperArg = "__devex-update-check"

// RunNotificationHelper handles the private subprocess entry point before normal
// CLI initialization. It never loads plugins, opens the database, or prints.
func RunNotificationHelper(ctx context.Context, args []string) bool {
	if len(args) == 0 || args[0] != notificationHelperArg {
		return false
	}
	if len(args) != 3 {
		return true
	}
	if _, err := ParseVersion(args[1]); err != nil {
		return true
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	client := NewClient()
	client.Token = os.Getenv("GITHUB_TOKEN")
	CheckNotification(ctx, client, args[1], args[2], time.Now())
	return true
}

func launchNotificationHelper(ctx context.Context, current, dir string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	// The child must outlive foreground cancellation. Its entry point applies a
	// separate bounded context; nil standard streams are connected to os.DevNull.
	cmd := exec.CommandContext(context.WithoutCancel(ctx), executable, notificationHelperArg, current, dir)
	detachNotificationHelper(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
