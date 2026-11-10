# Consent rules for app and plugin authors

DevEx must show users every install and system change before they agree to it.
Use these rules when reviewing application YAML and plugins:

1. Build previews from the same plan execution consumes. List packages, required
   dependencies (including mise and Docker), apt list refreshes, hook commands,
   rc changes, config destinations and exact desktop settings. Do not install
   additional tools from a post-install hook.
2. Scope effects to the selected app and shell. Use `shell_init` keyed by bash,
   zsh or fish and `path` for managed shell initialization. Do not run setup
   scripts with options such as `--all` that modify other shells. Optional
   settings need visible consent; desktop settings start disabled and use `s`
   on Confirmation to opt in. Unattended setup does not apply desktop settings.
3. Keep `default: true` apps in the visible Default tools section. Its checked
   entries are enabled together; `d` toggles the group and `--no-defaults`
   disables it for interactive or unattended setup.
4. Check whether an app is already installed; skip it and report the skip.
5. Back up user files and settings before writing, and record restoration data
   for undo. Use managed shell actions and backed-up config-file handling.
6. Report installed, skipped and failed items accurately. Propagate failures;
   successful package installation does not erase a failed configuration step.
7. Enforce these rules in validation and behavior tests with fake executors.
   Test success, skipped work, changed previews, failures and cancellation.

`InstallCommand.Validate` rejects raw rc appends and raw hooks referencing user
configuration (`~/.config`, `$XDG_CONFIG_HOME`, rc files, Git identity and SSH
configuration, including common HOME and absolute-home spellings). Structured
copy hooks cannot target those paths either. This conservative rule also rejects
reads in raw hooks: use `config_files` for configuration and managed shell actions
for initialization. OS validation recurses into alternatives on all platforms;
the directory loader and `devex config validate` apply it to shipped and custom YAML.

Static validation is a review guard, not a shell sandbox: computed paths, external
scripts and system-level effects still require human scope review. Never move a
forbidden write into a script or disguise its path to bypass validation. Config
file destinations must belong to the selected app.

Desktop plugins expose `describe` as JSON records without making changes. The
CLI displays these records, then passes their SHA-256 digest to `configure
--expected-plan`. Plugins must reject a changed plan before writing anything.
Existing GNOME/KDE plugins must support this contract before enabling their
settings in setup. Dry-run discovers installed plugins without downloading any;
when a supported preview is unavailable, desktop settings remain skipped.

The shipped catalog no longer bootstraps LazyVim or removes a Neovim config's
`.git` directory automatically. Install/configure LazyVim separately after
reviewing its setup instructions. Typora's theme/config file handler creates
parent directories; a raw mkdir hook is unnecessary. Homebrew fzf initialization
uses the selected shell's managed rc block instead of running its all-shell setup.
