package gnome

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/utils"
)

func SetBackground(imagePath string) error {
	command := fmt.Sprintf("gsettings set org.gnome.desktop.background picture-uri \"file://%s\"", imagePath)
	_, err := utils.CommandExec.RunShellCommand(command)
	return err
}
