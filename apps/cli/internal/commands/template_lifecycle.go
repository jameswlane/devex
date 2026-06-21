package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

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
	Force          bool
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
	plan, err := planTemplateLifecycle(ctx, planner, idOrPath, action, selectedRun, resources, knownResources, opts)
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
	if action == templates.LifecycleRemove {
		operations, err := store.ListOperationsByRun(ctx, result.RunID)
		if err != nil {
			return err
		}
		removedResourceIDs := removedResourceIDsForSuccessfulOperations(operations, resources)
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

func planTemplateLifecycle(ctx context.Context, planner templates.TemplatePlanner, idOrPath string, action templates.LifecycleAction, selectedRun state.TemplateRun, resources []state.ManagedResource, knownResources map[string]templates.ResourceDeclaration, opts templateLifecycleOptions) (*templates.ExecutionPlan, error) {
	inputOptions := templates.InputResolverOptions{
		Answers:        opts.Answers,
		AnswerFiles:    opts.AnswerFiles,
		NonInteractive: opts.NonInteractive,
	}
	if action == templates.LifecycleRemove {
		if plan, ok := recordedRemovePlan(selectedRun); ok {
			plan = prepareRecordedLifecyclePlan(plan, action, opts.Force)
			return &plan, nil
		}
	}

	plan, err := planner.Plan(ctx, idOrPath, templates.PlanOptions{
		Action:         action,
		KnownResources: knownResources,
		InputOptions:   inputOptions,
	})
	if err == nil {
		if action == templates.LifecycleRemove {
			prepared := prepareRecordedLifecyclePlan(*plan, action, opts.Force)
			if len(resources) > 0 {
				augmented, augmentErr := augmentRemovePlanWithRecordedResources(prepared, selectedRun, resources, opts.Force)
				if augmentErr != nil {
					return nil, augmentErr
				}
				prepared = augmented
			}
			return &prepared, nil
		}
		return plan, nil
	}
	if action != templates.LifecycleRemove || len(resources) == 0 {
		return nil, err
	}
	synthesized, synthErr := synthesizeRemovePlanFromResources(selectedRun, resources, opts.Force)
	if synthErr != nil {
		return nil, fmt.Errorf("%w; failed to synthesize remove plan from recorded resources: %v", err, synthErr)
	}
	return &synthesized, nil
}

func augmentRemovePlanWithRecordedResources(plan templates.ExecutionPlan, run state.TemplateRun, resources []state.ManagedResource, force bool) (templates.ExecutionPlan, error) {
	targeted := removedResourceIDsForPlan(plan, resources)
	var missing []state.ManagedResource
	for _, resource := range resources {
		if targeted[resource.ID] {
			continue
		}
		missing = append(missing, resource)
	}
	if len(missing) == 0 {
		return plan, nil
	}
	synthesized, err := synthesizeRemovePlanFromResources(run, missing, force)
	if err != nil {
		return templates.ExecutionPlan{}, err
	}
	nextOrder := len(plan.Steps) + 1
	for _, step := range synthesized.Steps {
		step.Order = nextOrder
		if step.ID != "" {
			step.ID = "recorded-" + step.ID
		}
		plan.Steps = append(plan.Steps, step)
		nextOrder++
	}
	return plan, nil
}

func recordedRemovePlan(run state.TemplateRun) (templates.ExecutionPlan, bool) {
	if run.Plan.Action != templates.LifecycleRemove || len(run.Plan.Steps) == 0 {
		return templates.ExecutionPlan{}, false
	}
	return run.Plan, true
}

func prepareRecordedLifecyclePlan(plan templates.ExecutionPlan, action templates.LifecycleAction, force bool) templates.ExecutionPlan {
	plan.ID = fmt.Sprintf("%s-%s-%d", plan.TemplateID, action, timeNow().UnixNano())
	plan.Action = action
	plan.CreatedAt = timeNow()
	if action == templates.LifecycleRemove && force {
		for index := range plan.Steps {
			plan.Steps[index].OnError = templates.ErrorPolicyContinue
			if plan.Steps[index].Metadata == nil {
				plan.Steps[index].Metadata = make(map[string]any)
			}
			plan.Steps[index].Metadata["force_remove"] = true
			plan.Steps[index].Metadata["audit"] = "forced template removal; failed operations are recorded and resource removal is audited"
		}
	}
	return plan
}

func synthesizeRemovePlanFromResources(run state.TemplateRun, resources []state.ManagedResource, force bool) (templates.ExecutionPlan, error) {
	if run.ID == "" {
		return templates.ExecutionPlan{}, fmt.Errorf("selected install run is required")
	}
	now := timeNow()
	steps := make([]templates.PlannedStep, 0, len(resources))
	for index, resource := range resources {
		request, ok := removeRequestForResource(resource)
		if !ok {
			return templates.ExecutionPlan{}, fmt.Errorf("resource '%s' does not include removal hints", resource.ID)
		}
		step := templates.PlannedStep{
			ID:          fmt.Sprintf("remove-%s", resource.StepID),
			Order:       index + 1,
			Type:        templates.StepTypePlugin,
			DisplayName: resource.Name,
			Request:     request,
			Resources: []templates.ResourceDeclaration{{
				Type:       resource.Type,
				Name:       resource.Name,
				ExternalID: resource.ExternalID,
				Metadata:   resource.Metadata,
			}},
			OnError: templates.ErrorPolicyStop,
			Metadata: map[string]any{
				"source_run_id": run.ID,
				"source":        "recorded-resource-removal-hints",
			},
		}
		if force {
			step.OnError = templates.ErrorPolicyContinue
			step.Metadata["force_remove"] = true
			step.Metadata["audit"] = "forced template removal synthesized from recorded resource state"
		}
		steps = append(steps, step)
	}
	return templates.ExecutionPlan{
		ID:              fmt.Sprintf("%s-remove-%d", run.TemplateID, now.UnixNano()),
		TemplateID:      run.TemplateID,
		TemplateVersion: run.TemplateVersion,
		Action:          templates.LifecycleRemove,
		Inputs:          run.Inputs,
		Steps:           steps,
		CreatedAt:       now,
	}, nil
}

func removeRequestForResource(resource state.ManagedResource) (templates.ExecutionRequest, bool) {
	removal, ok := mapFromAny(resource.Metadata["removal"])
	if !ok {
		return templates.ExecutionRequest{}, false
	}
	plugin := stringFromAny(removal["plugin"])
	command := stringFromAny(removal["command"])
	args := stringSliceFromAny(removal["args"])
	if plugin == "" {
		plugin = resource.Plugin
	}
	if command == "" {
		command = "remove"
	}
	if len(args) == 0 {
		target := resource.ExternalID
		if target == "" {
			target = resource.Name
		}
		if target != "" {
			args = []string{target}
		}
	}
	if plugin == "" || command == "" {
		return templates.ExecutionRequest{}, false
	}
	return templates.ExecutionRequest{
		Plugin:  plugin,
		Command: command,
		Args:    args,
		Metadata: map[string]any{
			"resource_id":          resource.ID,
			"resource_external_id": resource.ExternalID,
			"forced_audit":         removal["forced_audit"],
		},
	}, true
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
	if action == templates.LifecycleStatus {
		resources = sortTemplateResources(resources)
	} else {
		resources = filterActiveResources(resources)
	}
	if len(resources) == 0 {
		return state.TemplateRun{}, nil, fmt.Errorf("run '%s' has no recorded resources for template '%s'", run.ID, templateID)
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
		if run.Status != state.RunStatusSucceeded {
			return state.TemplateRun{}, fmt.Errorf("run '%s' has status '%s', not '%s'", run.ID, run.Status, state.RunStatusSucceeded)
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
	return sortTemplateResources(filtered)
}

func sortTemplateResources(resources []state.ManagedResource) []state.ManagedResource {
	ordered := append([]state.ManagedResource(nil), resources...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].StepID == ordered[j].StepID {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].StepID < ordered[j].StepID
	})
	return ordered
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

func removedResourceIDsForSuccessfulOperations(operations []state.OperationRecord, resources []state.ManagedResource) map[string]bool {
	removed := make(map[string]bool)
	for _, operation := range operations {
		if operation.Status != state.OperationStatusSucceeded {
			continue
		}
		if operation.Request.Command != string(templates.LifecycleRemove) && operation.Request.Command != "remove" && operation.Request.Command != "uninstall" {
			continue
		}
		step := templates.PlannedStep{
			ID:      operation.StepID,
			Request: operation.Request,
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

func mapFromAny(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	default:
		return nil, false
	}
}

func stringFromAny(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func stringSliceFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				values = append(values, text)
			}
		}
		return values
	default:
		return nil
	}
}

func timeNow() time.Time {
	return time.Now()
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
