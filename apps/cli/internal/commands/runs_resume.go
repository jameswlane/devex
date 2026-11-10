package commands

import (
	"fmt"
	"os"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templateexec"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newRunsResumeCmd(repo types.Repository, settings config.CrossPlatformSettings, provider pluginBootstrapProvider) *cobra.Command {
	var source string
	var answers, answerFiles []string
	cmd := &cobra.Command{Annotations: map[string]string{catalogAnnotation: catalogRequired}, Use: "resume <run-id>", Short: "Resume an interrupted installation with unchanged configuration", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if viper.GetBool("dry-run") {
			return fmt.Errorf("resume does not support --dry-run; inspect saved work with devex runs show %s", args[0])
		}
		store, err := openTemplateStateStore()
		if err != nil {
			return err
		}
		run, err := store.GetRun(cmd.Context(), args[0])
		if err != nil {
			_ = store.Close()
			return formatStateNotFound("run", args[0], err)
		}
		if run.Fingerprint == "" {
			_ = store.Close()
			return fmt.Errorf("run %s has no saved fingerprint; start a new install", run.ID)
		}
		if run.Status == state.RunStatusSucceeded {
			_ = store.Close()
			return fmt.Errorf("run %s already succeeded", run.ID)
		}
		if len(run.ApplicationNames) > 0 {
			_ = store.Close()

			apps := make([]types.CrossPlatformApp, 0, len(run.ApplicationNames))
			for _, name := range run.ApplicationNames {
				found := false
				for _, app := range settings.GetAllApps() {
					if app.Name == name {
						apps = append(apps, app)
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("application %s no longer exists in current configuration", name)
				}
			}
			fingerprint, err := state.ApplicationFingerprint(apps, settings)
			if err != nil {
				return err
			}
			if fingerprint != run.Fingerprint {
				return fmt.Errorf("configuration/plan fingerprint differs from run %s; restore the original configuration or start a new install", run.ID)
			}
			return runPlainInstall(state.WithResumeParent(cmd.Context(), run.ID), cmd.OutOrStdout(), apps, repo, settings, provider)
		}
		defer store.Close()
		if run.Action != templates.LifecycleInstall {
			return fmt.Errorf("resume currently supports installation runs only")
		}
		if source == "" {
			source = run.Source
		}
		if source == "" {
			return fmt.Errorf("supply --template with the original template path or id")
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		manager := templates.NewTemplateManager(home)
		planner := templates.TemplatePlanner{Manager: manager, AppResolver: templates.NewAppResolver(settings.GetApplicationByName)}
		plan, err := planner.Plan(cmd.Context(), source, templates.PlanOptions{Action: run.Action, InputOptions: templates.InputResolverOptions{Answers: answers, AnswerFiles: answerFiles, NonInteractive: true}})
		if err != nil {
			return err
		}
		fingerprint, err := templateexec.PlanFingerprint(*plan)
		if err != nil {
			return err
		}
		if fingerprint != run.Fingerprint {
			return fmt.Errorf("configuration/plan fingerprint differs from run %s; restore the original configuration and answers or start a new install", run.ID)
		}
		resumeProvider := provider
		if provider == nil || provider() == nil {
			pb, err := bootstrap.NewPluginBootstrap(viper.GetBool("skip-plugin-download") || viper.GetBool("offline"))
			if err != nil {
				return err
			}
			if err := pb.Initialize(cmd.Context()); err != nil {
				return err
			}
			resumeProvider = func() *bootstrap.PluginBootstrap { return pb }
		}
		executor, err := templatePluginExecutorResolver(resumeProvider)
		if err != nil {
			return err
		}
		runtime := templateexec.NewExecutionRuntime(store, executor)
		result, err := runtime.Execute(cmd.Context(), *plan, templateexec.ExecuteOptions{Concurrency: installConcurrency(), ResumeRunID: run.ID, Source: source})
		if result != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Resume attempt: %s\n", result.RunID)
		}
		return err
	}}
	cmd.Flags().StringVar(&source, "template", "", "Original template ID or path (defaults to saved source)")
	cmd.Flags().StringArrayVar(&answers, "answer", nil, "Original template answers (key=value)")
	cmd.Flags().StringArrayVar(&answerFiles, "answer-file", nil, "Files containing original template answers")
	return cmd
}
