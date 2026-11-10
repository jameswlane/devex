package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/interaction"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// customPlatforms maps flag prefixes to the platform keys used in application YAML.
var customPlatforms = []struct{ Flag, Key string }{
	{"linux", "linux"},
	{"macos", "macos"},
	{"windows", "windows"},
	{"all", "all_platforms"},
}

const (
	customDefaultCategory = "development"
	customPromptAttempts  = 3
)

var (
	customNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)
	// A package identifier is one argv element: letters, digits and the separators
	// package managers use (org.app.Name, owner/repo/formula, github:owner/repo).
	customPackagePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+@:/-]*$`)
	// Download methods need a URL plus checksum verification, which flags cannot
	// express safely yet; they stay available by editing the YAML directly.
	customDownloadMethods = map[string]bool{"curlpipe": true, "appimage": true}
)

// customPlatformInput is the user-supplied definition for one platform.
type customPlatformInput struct {
	Method      string
	Package     string
	PreInstall  []string
	PostInstall []string
}

func (in customPlatformInput) empty() bool {
	return in.Method == "" && in.Package == "" && len(in.PreInstall) == 0 && len(in.PostInstall) == 0
}

// customDefinition collects flag and prompt input before validation.
type customDefinition struct {
	Name        string
	Description string
	Category    string
	Platforms   map[string]*customPlatformInput // keyed by platform key
}

// customPlan is a validated definition and where it will be written.
type customPlan struct {
	App      types.CrossPlatformApp
	Path     string
	Replaces bool
}

// registerCustomFlags adds the flags that define a custom tool. --category and
// --dry-run are shared with the browse mode.
func registerCustomFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.Bool("custom", false, "Define a custom tool that is not in the catalog (name is the first argument)")
	flags.Bool("force", false, "Replace an existing definition of the same name (a backup is taken first)")
	flags.String("description", "", "Description of the custom tool (with --custom)")
	for _, platform := range customPlatforms {
		flags.String(platform.Flag+"-method", "", fmt.Sprintf("Install method for %s, e.g. apt, brew, mise (with --custom)", platform.Key))
		flags.String(platform.Flag+"-package", "", fmt.Sprintf("Package identifier for %s; one value, never shell text (with --custom)", platform.Key))
		flags.StringArray(platform.Flag+"-pre-install", nil, fmt.Sprintf("Shell command to run before installing on %s; repeatable (with --custom)", platform.Key))
		flags.StringArray(platform.Flag+"-post-install", nil, fmt.Sprintf("Shell command to run after installing on %s; repeatable (with --custom)", platform.Key))
	}
	_ = viper.BindPFlag("add.force", flags.Lookup("force"))
}

// customFlagNames lists flags that only make sense with --custom.
func customFlagNames() []string {
	names := make([]string, 0, 2+4*len(customPlatforms))
	names = append(names, "description", "force")
	for _, platform := range customPlatforms {
		for _, suffix := range []string{"-method", "-package", "-pre-install", "-post-install"} {
			names = append(names, platform.Flag+suffix)
		}
	}
	return names
}

// readCustomDefinition collects the definition from flags.
func readCustomDefinition(cmd *cobra.Command, args []string) (customDefinition, error) {
	flags := cmd.Flags()
	definition := customDefinition{Platforms: map[string]*customPlatformInput{}}
	if len(args) > 0 {
		definition.Name = strings.TrimSpace(args[0])
	}
	var err error
	if definition.Description, err = flags.GetString("description"); err != nil {
		return definition, err
	}
	if definition.Category, err = flags.GetString("category"); err != nil {
		return definition, err
	}
	definition.Description = strings.TrimSpace(definition.Description)
	definition.Category = strings.TrimSpace(definition.Category)
	for _, platform := range customPlatforms {
		input := &customPlatformInput{}
		if input.Method, err = flags.GetString(platform.Flag + "-method"); err != nil {
			return definition, err
		}
		if input.Package, err = flags.GetString(platform.Flag + "-package"); err != nil {
			return definition, err
		}
		if input.PreInstall, err = flags.GetStringArray(platform.Flag + "-pre-install"); err != nil {
			return definition, err
		}
		if input.PostInstall, err = flags.GetStringArray(platform.Flag + "-post-install"); err != nil {
			return definition, err
		}
		input.Method = strings.ToLower(strings.TrimSpace(input.Method))
		input.Package = strings.TrimSpace(input.Package)
		if !input.empty() {
			definition.Platforms[platform.Key] = input
		}
	}
	return definition, nil
}

// customPrompter asks questions on stderr and reads answers from one shared
// reader, so buffered input is not lost between questions.
type customPrompter struct {
	in  *bufio.Reader
	out io.Writer
}

func (p *customPrompter) ask(label, fallback string) (string, error) {
	if fallback != "" {
		fmt.Fprintf(p.out, "%s [%s]: ", label, fallback)
	} else {
		fmt.Fprintf(p.out, "%s: ", label)
	}
	line, err := p.in.ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || line == "") {
		return "", fmt.Errorf("read answer for %q: %w", label, err)
	}
	answer := strings.TrimSpace(line)
	if answer == "" {
		return fallback, nil
	}
	return answer, nil
}

// askValid re-asks a bounded number of times so a typo does not abort the form.
func (p *customPrompter) askValid(label, fallback string, validate func(string) error) (string, error) {
	var last error
	for attempt := 0; attempt < customPromptAttempts; attempt++ {
		answer, err := p.ask(label, fallback)
		if err != nil {
			return "", err
		}
		if last = validate(answer); last == nil {
			return answer, nil
		}
		fmt.Fprintf(p.out, "  %v\n", last)
	}
	return "", last
}

func (p *customPrompter) confirm(label string) (bool, error) {
	answer, err := p.ask(label+" [y/N]", "")
	if err != nil {
		return false, err
	}
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes"), nil
}

// fillCustomDefinition prompts for the fields flags left out. Optional hooks are
// only offered for platforms whose method or package had to be asked for.
func fillCustomDefinition(p *customPrompter, definition *customDefinition) error {
	var err error
	if definition.Name == "" {
		if definition.Name, err = p.askValid("Application name", "", validateCustomName); err != nil {
			return err
		}
	}
	if definition.Description == "" {
		if definition.Description, err = p.ask("Description", "Custom application "+definition.Name); err != nil {
			return err
		}
	}
	if definition.Category == "" {
		if definition.Category, err = p.ask("Category", customDefaultCategory); err != nil {
			return err
		}
	}
	if len(definition.Platforms) == 0 {
		answer, err := p.askValid("Platforms to configure (linux, macos, windows, all)", currentPlatformFlag(), func(value string) error {
			_, err := parseCustomPlatforms(value)
			return err
		})
		if err != nil {
			return err
		}
		keys, _ := parseCustomPlatforms(answer)
		for _, key := range keys {
			definition.Platforms[key] = &customPlatformInput{}
		}
	}
	for _, platform := range customPlatforms {
		input, ok := definition.Platforms[platform.Key]
		if !ok {
			continue
		}
		asked := false
		if input.Method == "" {
			asked = true
			supported := strings.Join(customSupportedMethods(platform.Key), ", ")
			method, err := p.askValid(fmt.Sprintf("%s install method (%s)", platform.Key, supported), "", func(value string) error {
				return validateCustomMethod(platform.Key, strings.ToLower(strings.TrimSpace(value)))
			})
			if err != nil {
				return err
			}
			input.Method = strings.ToLower(strings.TrimSpace(method))
		}
		if input.Package == "" {
			asked = true
			pkg, err := p.askValid(platform.Key+" package identifier", "", func(value string) error { return validateCustomPackage(input.Method, strings.TrimSpace(value)) })
			if err != nil {
				return err
			}
			input.Package = strings.TrimSpace(pkg)
		}
		if asked && len(input.PreInstall) == 0 && len(input.PostInstall) == 0 {
			pre, err := p.ask(platform.Key+" pre-install command (optional, blank for none)", "")
			if err != nil {
				return err
			}
			if pre != "" {
				input.PreInstall = []string{pre}
			}
			post, err := p.ask(platform.Key+" post-install command (optional, blank for none)", "")
			if err != nil {
				return err
			}
			if post != "" {
				input.PostInstall = []string{post}
			}
		}
	}
	return nil
}

func currentPlatformFlag() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	default:
		return "linux"
	}
}

// parseCustomPlatforms converts "linux, macos" into platform keys.
func parseCustomPlatforms(value string) ([]string, error) {
	var keys []string
	for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' }) {
		found := false
		for _, platform := range customPlatforms {
			if strings.EqualFold(part, platform.Flag) || strings.EqualFold(part, platform.Key) {
				keys = append(keys, platform.Key)
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown platform %q; use linux, macos, windows or all", part)
		}
	}
	if len(keys) == 0 {
		return nil, errors.New("choose at least one platform: linux, macos, windows or all")
	}
	return keys, nil
}

func validateCustomName(name string) error {
	if !customNamePattern.MatchString(name) {
		return fmt.Errorf("application name %q is invalid; start with a letter or digit and use only letters, digits, '.', '_', '+' and '-' (64 characters at most)", name)
	}
	return nil
}

// customSupportedMethods lists plugin-backed methods --custom can define for a platform.
func customSupportedMethods(platformKey string) []string {
	var methods []string
	for _, method := range config.SupportedInstallMethods(platformKey) {
		if !customDownloadMethods[method] {
			methods = append(methods, method)
		}
	}
	return methods
}

// validateCustomMethod accepts only install methods that have a package-manager
// plugin for the platform, and lists the supported ones otherwise.
func validateCustomMethod(platformKey, method string) error {
	supported := strings.Join(customSupportedMethods(platformKey), ", ")
	switch config.ClassifyInstallMethod(method) {
	case config.MethodSupported:
		if customDownloadMethods[method] {
			return fmt.Errorf("%s: install method %q downloads artifacts and needs checksum verification, which --custom cannot express; edit the application YAML instead (supported methods: %s)", platformKey, method, supported)
		}
		if !config.InstallMethodSupportsPlatform(method, platformKey) {
			return fmt.Errorf("%s: install method %q is not available on this platform; supported methods: %s", platformKey, method, supported)
		}
		return nil
	case config.MethodPlanned:
		return fmt.Errorf("%s: install method %q is planned but has no plugin yet; supported methods: %s", platformKey, method, supported)
	default:
		if method == "" {
			return fmt.Errorf("%s: install method is required; supported methods: %s", platformKey, supported)
		}
		return fmt.Errorf("%s: unknown install method %q; supported methods: %s", platformKey, method, supported)
	}
}

// validateCustomPackage keeps the identifier a single argv element. Commands
// belong in the pre/post-install hook flags, never in the package value.
func validateCustomPackage(method, pkg string) error {
	switch {
	case pkg == "":
		return errors.New("package identifier is required")
	case strings.ContainsAny(pkg, " \t\r\n"):
		return fmt.Errorf("package identifier %q contains whitespace; give one package per value and use the pre-install/post-install flags for commands", pkg)
	case strings.HasPrefix(pkg, "-"):
		return fmt.Errorf("package identifier %q starts with '-' and would be read as an option", pkg)
	case !customPackagePattern.MatchString(pkg):
		return fmt.Errorf("package identifier %q contains characters that are not valid in a package name (shell metacharacters are never allowed)", pkg)
	}
	return nil
}

// validateCustomHook applies the same install-hook checks that run at install time.
func validateCustomHook(settings config.CrossPlatformSettings, appName, script string) error {
	if strings.TrimSpace(script) == "" {
		return errors.New("hook command is empty")
	}
	if strings.ContainsRune(script, 0) {
		return errors.New("hook command contains a NUL byte")
	}
	if err := (types.InstallCommand{Shell: script}).Validate(); err != nil {
		return err
	}
	validator, err := settings.GetCommandValidator()
	if err != nil {
		return err
	}
	if err := validator.ValidateCommandForApp(script, appName); err != nil {
		return fmt.Errorf("rejected by the command policy: %w; review the hook or the security.yaml level/app_overrides", err)
	}
	return nil
}

// buildCustomApp validates the definition and converts it to the stored model.
// The package stays in install_command; hooks are separate shell entries.
func buildCustomApp(settings config.CrossPlatformSettings, definition customDefinition) (types.CrossPlatformApp, error) {
	var problems []error
	if err := validateCustomName(definition.Name); err != nil {
		problems = append(problems, err)
	}
	app := types.CrossPlatformApp{Name: definition.Name, Description: definition.Description, Category: definition.Category}
	if app.Description == "" {
		app.Description = "Custom application " + app.Name
	}
	if app.Category == "" {
		app.Category = customDefaultCategory
	}
	if configurationPathPart(app.Category) == "" {
		problems = append(problems, fmt.Errorf("category %q needs at least one letter or digit", app.Category))
	}
	if len(definition.Platforms) == 0 {
		problems = append(problems, errors.New("define at least one platform, e.g. --linux-method apt --linux-package NAME"))
	}
	for _, platform := range customPlatforms {
		input, ok := definition.Platforms[platform.Key]
		if !ok {
			continue
		}
		osConfig := types.OSConfig{InstallMethod: input.Method, InstallCommand: input.Package}
		if err := validateCustomMethod(platform.Key, input.Method); err != nil {
			problems = append(problems, err)
		} else if err := validateCustomPackage(input.Method, input.Package); err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", platform.Key, err))
		}
		osConfig.UninstallCommand = input.Package
		if input.Method == "brew" {
			osConfig.UninstallCommand = path.Base(input.Package)
		}
		for _, hook := range []struct {
			name     string
			scripts  []string
			destList *[]types.InstallCommand
		}{
			{"pre-install", input.PreInstall, &osConfig.PreInstall},
			{"post-install", input.PostInstall, &osConfig.PostInstall},
		} {
			for _, script := range hook.scripts {
				if err := validateCustomHook(settings, app.Name, script); err != nil {
					problems = append(problems, fmt.Errorf("%s %s hook %q: %w", platform.Key, hook.name, script, err))
					continue
				}
				*hook.destList = append(*hook.destList, types.InstallCommand{Shell: script})
			}
		}
		switch platform.Key {
		case "linux":
			app.Linux = osConfig
		case "macos":
			app.MacOS = osConfig
		case "windows":
			app.Windows = osConfig
		default:
			app.AllPlatforms = osConfig
		}
	}
	if err := errors.Join(problems...); err != nil {
		return types.CrossPlatformApp{}, fmt.Errorf("invalid custom application: %w", err)
	}
	return app, nil
}

// planCustomAddition resolves where the definition goes and refuses to touch an
// existing definition unless force is set. It changes nothing on disk.
func planCustomAddition(ctx context.Context, settings config.CrossPlatformSettings, app types.CrossPlatformApp, force bool) (customPlan, error) {
	if err := ctx.Err(); err != nil {
		return customPlan{}, err
	}
	configured, err := config.LoadApplicationConfigs(settings.GetConfigDir())
	if err != nil {
		return customPlan{}, err
	}
	replacing := map[string]bool{}
	var kept []types.CrossPlatformApp
	for _, existing := range configured {
		if strings.EqualFold(existing.Name, app.Name) {
			if !force {
				return customPlan{}, fmt.Errorf("application %q is already in your configuration; use --force to replace it (the configuration is backed up first)", app.Name)
			}
			replacing[strings.ToLower(app.Name)] = true
			continue
		}
		kept = append(kept, existing)
	}
	if !replacing[strings.ToLower(app.Name)] {
		for _, shipped := range settings.GetAllApps() {
			if strings.EqualFold(shipped.Name, app.Name) {
				if !force {
					return customPlan{}, fmt.Errorf("application %q already exists in the catalog; choose another name, or use --force to override it with your custom definition", app.Name)
				}
				replacing[strings.ToLower(app.Name)] = true
			}
		}
	}
	if err := checkPlanCollisions(settings, configured, []types.CrossPlatformApp{app}, replacing); err != nil {
		return customPlan{}, err
	}
	if err := validatePlan(ctx, settings, kept, []types.CrossPlatformApp{app}, len(replacing) > 0); err != nil {
		return customPlan{}, err
	}
	target, err := applicationConfigPath(settings.GetConfigDir(), "applications", app.Category, app.Name)
	if err != nil {
		return customPlan{}, err
	}
	return customPlan{App: app, Path: target, Replaces: len(replacing) > 0}, nil
}

// customAddOptions are the non-definition inputs of a custom add.
type customAddOptions struct {
	DryRun, Yes, Force bool
	Format             string
}

// runAddCustom defines a custom tool from flags and, when interactive, prompts.
// It never runs a package manager: it only writes configuration.
func runAddCustom(cmd *cobra.Command, args []string, settings config.CrossPlatformSettings, opts customAddOptions) error {
	if opts.Format != "table" && opts.Format != "json" {
		return fmt.Errorf("unsupported format %q; use table or json", opts.Format)
	}
	definition, err := readCustomDefinition(cmd, args)
	if err != nil {
		return err
	}
	interactive := interaction.IsInteractiveMode() && opts.Format != "json"
	prompter := &customPrompter{in: bufio.NewReader(cmd.InOrStdin()), out: cmd.ErrOrStderr()}
	if interactive {
		if err := fillCustomDefinition(prompter, &definition); err != nil {
			return err
		}
	} else if definition.Name == "" {
		return errors.New("specify an application name with --custom (devex add NAME --custom ...); prompts are unavailable in unattended mode")
	}
	app, err := buildCustomApp(settings, definition)
	if err != nil {
		return err
	}
	plan, err := planCustomAddition(cmd.Context(), settings, app, opts.Force)
	if err != nil {
		return err
	}
	preview, err := yaml.Marshal(plan.App)
	if err != nil {
		return fmt.Errorf("render preview for %q: %w", app.Name, err)
	}
	verb := "add"
	if plan.Replaces {
		verb = "replace"
	}
	if opts.Format == "table" {
		out := cmd.ErrOrStderr()
		if opts.DryRun {
			out = cmd.OutOrStdout()
		}
		fmt.Fprintf(out, "Configuration preview: %s custom application %s in %s\n---\n%s---\n", verb, app.Name, plan.Path, preview)
	}
	if !opts.DryRun && !opts.Yes {
		if !interactive {
			return fmt.Errorf("confirmation required; review with --dry-run, then use --yes to %s the custom application", verb)
		}
		ok, err := prompter.confirm(strings.ToUpper(verb[:1]) + verb[1:] + " this application?")
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("addition cancelled; configuration unchanged")
		}
	}
	if err := cmd.Context().Err(); err != nil {
		return err
	}
	if !opts.DryRun {
		if err := saveApplications(settings, []types.CrossPlatformApp{plan.App}, plan.Replaces); err != nil {
			return err
		}
	}
	if opts.Format == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
			Added    []string `json:"added"`
			DryRun   bool     `json:"dry_run"`
			Replaced bool     `json:"replaced"`
			Path     string   `json:"path"`
			YAML     string   `json:"yaml"`
		}{[]string{app.Name}, opts.DryRun, plan.Replaces, plan.Path, string(preview)})
	}
	switch {
	case opts.DryRun:
		fmt.Fprintf(cmd.OutOrStdout(), "Would %s: %s\n", verb, app.Name)
	case plan.Replaces:
		fmt.Fprintf(cmd.OutOrStdout(), "Replaced: %s (%s)\n", app.Name, plan.Path)
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Added: %s (%s)\n", app.Name, plan.Path)
	}
	return nil
}

// rejectCustomFlagsWithoutCustom stops silent misuse of definition flags.
func rejectCustomFlagsWithoutCustom(cmd *cobra.Command) error {
	var used []string
	for _, name := range customFlagNames() {
		if cmd.Flags().Changed(name) {
			used = append(used, "--"+name)
		}
	}
	if len(used) > 0 {
		return fmt.Errorf("%s only apply with --custom; add --custom to define a tool that is not in the catalog", strings.Join(used, ", "))
	}
	return nil
}
