package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templateexec"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
)

type templateLifecycleOptions struct {
	Answers        []string
	AnswerFiles    []string
	NonInteractive bool
	DryRun         bool
	Format         string
	Run            string
}

type templateLifecycleResult struct {
	RunID         string                    `json:"run_id,omitempty" yaml:"run_id,omitempty"`
	SelectedRunID string                    `json:"selected_run_id,omitempty" yaml:"selected_run_id,omitempty"`
	TemplateID    string                    `json:"template_id" yaml:"template_id"`
	Action        templates.LifecycleAction `json:"action" yaml:"action"`
	DryRun        bool                      `json:"dry_run" yaml:"dry_run"`
	StepsPlanned  int                       `json:"steps_planned" yaml:"steps_planned"`
	StepsExecuted int                       `json:"steps_executed" yaml:"steps_executed"`
	Resources     []state.ManagedResource   `json:"resources,omitempty" yaml:"resources,omitempty"`
}

func executeTemplateLifecycle(ctx context.Context, idOrPath string, action templates.LifecycleAction, opts templateLifecycleOptions, settings config.CrossPlatformSettings, getPluginBootstrap pluginBootstrapProvider) error {
	if idOrPath == "" {
		return fmt.Errorf("--template is required")
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine user home directory: %w", err)
	}

	manager := templates.NewTemplateManager(homeDir)
	spec, err := manager.GetTemplateSpecByIDOrPath(idOrPath)
	if err != nil {
		return err
	}
	if err := manager.ValidateTemplateSpec(*spec); err != nil {
		return err
	}

	store, err := openTemplateStateStore()
	if err != nil {
		return err
	}
	defer store.Close()

	selectedRun, resources, err := selectTemplateRunResources(ctx, store, spec.ID, action, opts.Run)
	if err != nil {
		return err
	}
	knownResources := knownResourcesByStep(resources)

	planner := templates.TemplatePlanner{
		Manager:     manager,
		AppResolver: templates.NewAppResolver(settings.GetApplicationByName),
	}
	plan, err := planner.Plan(ctx, idOrPath, templates.PlanOptions{
		Action:         action,
		KnownResources: knownResources,
		InputOptions: templates.InputResolverOptions{
			Answers:        opts.Answers,
			AnswerFiles:    opts.AnswerFiles,
			NonInteractive: opts.NonInteractive,
		},
	})
	if err != nil {
		if action == templates.LifecycleStatus && len(resources) > 0 {
			return displayRecordedTemplateResources(templateLifecycleResult{
				SelectedRunID: selectedRun.ID,
				TemplateID:    spec.ID,
				Action:        action,
				Resources:     resources,
			}, opts.Format)
		}
		return err
	}

	if opts.DryRun {
		redactedPlan := templates.RedactPlan(*spec, *plan)
		return displayExecutionPlan(&redactedPlan, opts.Format)
	}

	pluginExecutor, err := resolveTemplatePluginExecutor(getPluginBootstrap)
	if err != nil {
		return err
	}
	runtime := templateexec.NewExecutionRuntime(store, pluginExecutor)
	result, err := runtime.Execute(ctx, *plan, templateexec.ExecuteOptions{})
	if err != nil {
		return err
	}
	if action == templates.LifecycleRemove {
		removedResourceIDs := removedResourceIDsForPlan(*plan, resources)
		for index, resource := range resources {
			if resource.CurrentState == state.ResourceStateRemoved {
				continue
			}
			if !removedResourceIDs[resource.ID] {
				continue
			}
			if err := store.MarkResourceRemoved(ctx, resource.ID); err != nil {
				return err
			}
			resources[index].CurrentState = state.ResourceStateRemoved
		}
	}

	return displayTemplateLifecycleResult(templateLifecycleResult{
		RunID:         result.RunID,
		SelectedRunID: selectedRun.ID,
		TemplateID:    plan.TemplateID,
		Action:        action,
		StepsPlanned:  result.StepsPlanned,
		StepsExecuted: result.StepsExecuted,
		Resources:     resources,
	}, opts.Format)
}

func openTemplateStateStore() (state.StateStore, error) {
	dbPath, err := state.ConfiguredDBPath()
	if err != nil {
		return nil, err
	}
	return state.OpenBoltStore(dbPath, 0o600)
}

func resolveTemplatePluginExecutor(getPluginBootstrap pluginBootstrapProvider) (templateexec.PluginExecutor, error) {
	if getPluginBootstrap != nil {
		if pluginBootstrap := getPluginBootstrap(); pluginBootstrap != nil {
			return pluginBootstrapExecutor{bootstrap: pluginBootstrap}, nil
		}
	}
	return nil, fmt.Errorf("plugin runtime is unavailable for template lifecycle action")
}

func selectTemplateRunResources(ctx context.Context, store state.StateStore, templateID string, action templates.LifecycleAction, runRef string) (state.TemplateRun, []state.ManagedResource, error) {
	if action == templates.LifecycleInstall {
		return state.TemplateRun{}, nil, nil
	}
	run, err := selectTemplateInstallRun(ctx, store, templateID, runRef)
	if err != nil {
		return state.TemplateRun{}, nil, err
	}
	resources, err := store.ListResourcesByRun(ctx, run.ID)
	if err != nil {
		return state.TemplateRun{}, nil, err
	}
	resources = filterActiveResources(resources)
	if len(resources) == 0 {
		return state.TemplateRun{}, nil, fmt.Errorf("run '%s' has no active resources for template '%s'", run.ID, templateID)
	}
	return run, resources, nil
}

func selectTemplateInstallRun(ctx context.Context, store state.StateStore, templateID string, runRef string) (state.TemplateRun, error) {
	if runRef != "" && runRef != "latest" {
		run, err := store.GetRun(ctx, runRef)
		if err != nil {
			return state.TemplateRun{}, err
		}
		if run.TemplateID != templateID {
			return state.TemplateRun{}, fmt.Errorf("run '%s' belongs to template '%s', not '%s'", run.ID, run.TemplateID, templateID)
		}
		if run.Action != templates.LifecycleInstall {
			return state.TemplateRun{}, fmt.Errorf("run '%s' is a '%s' run, not an install run", run.ID, run.Action)
		}
		return *run, nil
	}

	runs, err := store.ListRuns(ctx, state.RunFilter{
		TemplateID: templateID,
		Action:     templates.LifecycleInstall,
		Status:     state.RunStatusSucceeded,
	})
	if err != nil {
		return state.TemplateRun{}, err
	}
	if len(runs) == 0 {
		return state.TemplateRun{}, fmt.Errorf("no successful install run found for template '%s'", templateID)
	}
	sort.SliceStable(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})
	return runs[0], nil
}

func filterActiveResources(resources []state.ManagedResource) []state.ManagedResource {
	filtered := make([]state.ManagedResource, 0, len(resources))
	for _, resource := range resources {
		if resource.CurrentState == state.ResourceStateRemoved || resource.RemovedAt != nil {
			continue
		}
		filtered = append(filtered, resource)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].StepID == filtered[j].StepID {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].StepID < filtered[j].StepID
	})
	return filtered
}

func knownResourcesByStep(resources []state.ManagedResource) map[string]templates.ResourceDeclaration {
	known := make(map[string]templates.ResourceDeclaration, len(resources))
	for _, resource := range resources {
		if resource.StepID == "" {
			continue
		}
		if _, exists := known[resource.StepID]; exists {
			continue
		}
		known[resource.StepID] = templates.ResourceDeclaration{
			Type:       resource.Type,
			Name:       resource.Name,
			ExternalID: resource.ExternalID,
			Metadata:   resource.Metadata,
		}
	}
	return known
}

func removedResourceIDsForPlan(plan templates.ExecutionPlan, resources []state.ManagedResource) map[string]bool {
	removed := make(map[string]bool)
	for _, step := range plan.Steps {
		if step.Request.Command != string(templates.LifecycleRemove) && step.Request.Command != "remove" && step.Request.Command != "uninstall" {
			continue
		}
		for _, resource := range resources {
			if removeStepTargetsResource(step, resource) {
				removed[resource.ID] = true
			}
		}
	}
	return removed
}

func removeStepTargetsResource(step templates.PlannedStep, resource state.ManagedResource) bool {
	if step.ID == resource.StepID {
		return true
	}
	for _, declaration := range step.Resources {
		if declaration.ExternalID != "" && declaration.ExternalID == resource.ExternalID {
			return true
		}
		if declaration.Name != "" && declaration.Name == resource.Name && (declaration.Type == "" || declaration.Type == resource.Type) {
			return true
		}
	}
	for _, value := range requestTargetValues(step.Request) {
		if value == "" {
			continue
		}
		if resource.ExternalID != "" && value == resource.ExternalID {
			return true
		}
		if resource.Name != "" && value == resource.Name {
			return true
		}
	}
	return false
}

func requestTargetValues(request templates.ExecutionRequest) []string {
	values := append([]string(nil), request.Args...)
	for _, arg := range request.Args {
		for _, separator := range []string{"=", ":"} {
			if before, after, found := splitOnce(arg, separator); found && before != "" && after != "" {
				values = append(values, after)
			}
		}
	}
	return values
}

func splitOnce(value string, separator string) (string, string, bool) {
	for index := 0; index+len(separator) <= len(value); index++ {
		if value[index:index+len(separator)] == separator {
			return value[:index], value[index+len(separator):], true
		}
	}
	return "", "", false
}

func displayRecordedTemplateResources(result templateLifecycleResult, format string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	case "yaml":
		data, err := yaml.Marshal(result)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		fmt.Println(string(data))
	default:
		fmt.Printf("Recorded resources for template %s\n", result.TemplateID)
		if result.SelectedRunID != "" {
			fmt.Printf("Selected install run: %s\n", result.SelectedRunID)
		}
		printTemplateResourceTable(result.Resources)
	}
	return nil
}

func displayTemplateLifecycleResult(result templateLifecycleResult, format string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	case "yaml":
		data, err := yaml.Marshal(result)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		fmt.Println(string(data))
	default:
		fmt.Printf("Template %s run %s completed: %d/%d step(s) executed\n", result.Action, result.RunID, result.StepsExecuted, result.StepsPlanned)
		if result.SelectedRunID != "" {
			fmt.Printf("Selected install run: %s\n", result.SelectedRunID)
		}
		printTemplateResourceTable(result.Resources)
	}
	return nil
}

func printTemplateResourceTable(resources []state.ManagedResource) {
	if len(resources) == 0 {
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "STEP\tTYPE\tNAME\tEXTERNAL ID\tSTATE")
	fmt.Fprintln(w, "----\t----\t----\t-----------\t-----")
	for _, resource := range resources {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", resource.StepID, resource.Type, resource.Name, resource.ExternalID, resource.CurrentState)
	}
	_ = w.Flush()
}
