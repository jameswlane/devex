package commands

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/pkg/datastore/repository"
)

func CreateSystemCommand(systemRepo repository.SystemRepository) *cobra.Command {
	return &cobra.Command{
		Use:   "system",
		Short: "Manage system data",
		Run: func(cmd *cobra.Command, args []string) {
			data, err := systemRepo.GetAll()
			if err != nil {
				log.Error("Failed to fetch system data", "error", err)
				return
			}
			for key, value := range data {
				fmt.Printf("%s: %s\n", key, value)
			}
		},
	}
}
