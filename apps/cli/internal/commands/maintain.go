package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func NewMaintainCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	return NewMaintainCmdWithPluginProvider(repo, settings, nil)
}

func NewMaintainCmdWithPluginProvider(_ types.Repository, settings config.CrossPlatformSettings, getPluginBootstrap pluginBootstrapProvider) *cobra.Command {
	var (
		templateID     string
		answers        []string
		answerFiles    []string
		nonInteractive bool
		format         string
		runRef         string
	)

	cmd := &cobra.Command{
		Use:   "maintain",
		Short: "Maintain resources created by lifecycle templates",
		Long: `Maintain resources created by DevEx lifecycle templates.

The maintain command selects a recorded install run for the template, loads its
active resources from the local state database, and executes the template's
maintain lifecycle steps against those resources.`,
		Example: `  # Maintain the latest successful install run for a template
  devex maintain --template databases

  # Maintain a specific recorded install run
  devex maintain --template databases --run databases-install-1234

  # Preview maintenance work
  devex maintain --template databases --dry-run`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if templateID == "" {
				return fmt.Errorf("--template is required")
			}
			return executeTemplateLifecycle(cmd.Context(), templateID, templates.LifecycleMaintain, templateLifecycleOptions{
				Answers:        answers,
				AnswerFiles:    answerFiles,
				NonInteractive: nonInteractive,
				DryRun:         viper.GetBool("dry-run"),
				Format:         format,
				Run:            runRef,
			}, settings, getPluginBootstrap)
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&templateID, "template", "", "Maintain a DevEx lifecycle template by ID or path")
	cmd.Flags().StringArrayVar(&answers, "answer", nil, "Template input answer in key=value form")
	cmd.Flags().StringArrayVar(&answerFiles, "answers", nil, "YAML file containing template input answers")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Fail instead of prompting for missing template inputs")
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json, yaml)")
	cmd.Flags().StringVar(&runRef, "run", "latest", "Template install run to maintain (latest or run ID)")

	_ = viper.BindPFlag("non-interactive", cmd.Flags().Lookup("non-interactive"))
	return cmd
}
