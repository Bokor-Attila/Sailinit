package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
)

var version = "dev"

// CommandRunner executes a command and streams output.
type CommandRunner func(name string, dir string, stdin string, args ...string) error

// CommandOutputRunner executes a command and returns its standard output.
type CommandOutputRunner func(name string, dir string, args ...string) ([]byte, error)

var defaultCommandRunner CommandRunner = func(name string, dir string, stdin string, args ...string) error {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	} else {
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

var defaultCommandOutputRunner CommandOutputRunner = func(name string, dir string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.Output()
}

var execRunner = defaultCommandRunner
var execOutputRunner = defaultCommandOutputRunner

func isDockerRunning() bool {
	_, err := execOutputRunner("docker", "", "info")
	return err == nil
}

func checkDockerDaemon() error {
	if !isDockerRunning() {
		return fmt.Errorf("Docker daemon is not running. Please start Docker Engine / Docker Desktop first.")
	}
	return nil
}

func detectPHPVersion(projectDir string) string {
	files := []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"}
	for _, f := range files {
		path := filepath.Join(projectDir, f)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		content := string(data)
		// Look for vendor runtimes path
		reRuntime := regexp.MustCompile(`runtimes/([0-9]+\.[0-9]+)`)
		match := reRuntime.FindStringSubmatch(content)
		if len(match) > 1 {
			return strings.ReplaceAll(match[1], ".", "")
		}

		// Look for image name
		reImage := regexp.MustCompile(`sail-([0-9]+\.[0-9]+)/app`)
		match = reImage.FindStringSubmatch(content)
		if len(match) > 1 {
			return strings.ReplaceAll(match[1], ".", "")
		}

		// Look for docker context path (alternative)
		reDocker := regexp.MustCompile(`context: \.?/docker/([0-9]+\.[0-9]+)`)
		match = reDocker.FindStringSubmatch(content)
		if len(match) > 1 {
			return strings.ReplaceAll(match[1], ".", "")
		}
	}
	return ""
}

func checkLaravelProject(projectDir string, yesFlag bool) {
	composerPath := filepath.Join(projectDir, "composer.json")
	artisanPath := filepath.Join(projectDir, "artisan")
	_, errComposer := os.Stat(composerPath)
	_, errArtisan := os.Stat(artisanPath)

	if os.IsNotExist(errComposer) && os.IsNotExist(errArtisan) {
		printWarning("Warning: No Laravel project files (composer.json or artisan) detected in current directory.")
		if !confirmOrAbort(
			"no Laravel project (composer.json or artisan) found in "+projectDir,
			"re-run with -y to set up here anyway, or cd into a Laravel project",
			yesFlag) {
			os.Exit(exitAborted)
		}
	}
}

func generateCompletion(shell string) {
	switch strings.ToLower(shell) {
	case "bash":
		fmt.Fprint(stdout, `# bash completion for sailinit
_sailinit() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    opts="--version -v --list -l --status -s --clean -c --remove -r --stop --down --fresh -f --reset-db --dry-run -d --yes -y --non-interactive --new -n --with -w --json -j --port -p --ports --upgrade -u --open -o --doctor --install-skill --uninstall-skill --refresh-skill --completion"

    if [[ ${cur} == -* ]] ; then
        COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
        return 0
    fi
}
complete -F _sailinit sailinit
`)
	case "zsh":
		fmt.Fprint(stdout, `# zsh completion for sailinit
#compdef sailinit

_sailinit() {
    local -a options
    options=(
        '(-v --version)'{-v,--version}'[Print version and exit]'
        '(-l --list)'{-l,--list}'[List all registered projects]'
        '(-s --status)'{-s,--status}'[Show container status for registered projects]'
        '(-c --clean)'{-c,--clean}'[Remove orphaned projects from registry]'
        '(-r --remove)'{-r,--remove}'[Remove current project from port registry]'
        '--stop[Run sail stop in current project]'
        '--down[Run sail down in current project]'
        '(-f --fresh)'{-f,--fresh}'[Force re-run composer install]'
        '--reset-db[Reset DB settings to Sail defaults]'
        '(-d --dry-run)'{-d,--dry-run}'[Preview changes without modifying system]'
        '(-y --yes --non-interactive)'{-y,--yes,--non-interactive}'[Non-interactive mode; auto-confirm prompts]'
        '(-n --new)'{-n,--new}'[Create new Laravel project]:project name:'
        '(-w --with)'{-w,--with}'[Services to include for new project]:services:'
        '(-j --json)'{-j,--json}'[Output in JSON format]'
        '(-p --port)'{-p,--port}'[Print APP_PORT for current project]'
        '--ports[Print every calculated port for the current project]'
        '(-u --upgrade)'{-u,--upgrade}'[Download and install the latest release]'
        '(-o --open)'{-o,--open}'[Open the current project URL in the browser]'
        '--doctor[Run diagnostics on the registry and current project]'
        '--install-skill[Install the Claude Code skill for sailinit]'
        '--uninstall-skill[Remove the Claude Code skill]'
        '--refresh-skill[Update an installed Claude Code skill to this version]'
        '--completion[Generate shell completion script]:shell:(bash zsh fish)'
    )
    _describe -t commands 'sailinit flags' options
}

_sailinit "$@"
`)
	case "fish":
		fmt.Fprint(stdout, `# fish completion for sailinit
complete -c sailinit -s v -l version -d 'Print version and exit'
complete -c sailinit -s l -l list -d 'List all registered projects'
complete -c sailinit -s s -l status -d 'Show status of all registered projects'
complete -c sailinit -s c -l clean -d 'Remove entries for non-existent projects'
complete -c sailinit -s r -l remove -d 'Remove current project from registry'
complete -c sailinit -l stop -d 'Run sail stop in current project'
complete -c sailinit -l down -d 'Run sail down in current project'
complete -c sailinit -s f -l fresh -d 'Force re-run composer install'
complete -c sailinit -l reset-db -d 'Reset DB settings to defaults'
complete -c sailinit -s d -l dry-run -d 'Show what would happen without making changes'
complete -c sailinit -s y -l yes -l non-interactive -d 'Automatic yes to prompts'
complete -c sailinit -s n -l new -r -d 'Create a new Laravel project'
complete -c sailinit -s w -l with -r -d 'Services to include (default: mysql)'
complete -c sailinit -s j -l json -d 'Output in JSON format'
complete -c sailinit -s p -l port -d 'Print APP_PORT for current project'
complete -c sailinit -l ports -d 'Print every calculated port for the current project'
complete -c sailinit -s u -l upgrade -d 'Download and install the latest release'
complete -c sailinit -s o -l open -d 'Open the current project URL in the browser'
complete -c sailinit -l doctor -d 'Run diagnostics on the registry and current project'
complete -c sailinit -l install-skill -d 'Install the Claude Code skill for sailinit'
complete -c sailinit -l uninstall-skill -d 'Remove the Claude Code skill'
complete -c sailinit -l refresh-skill -d 'Update an installed Claude Code skill to this version'
complete -c sailinit -l completion -r -f -a 'bash zsh fish' -d 'Generate shell completion script'
`)
	default:
		printError(fmt.Sprintf("Unknown shell: %s. Supported shells: bash, zsh, fish", shell))
		os.Exit(exitUsage)
	}
}

func main() {
	var (
		versionFlag        bool
		listFlag           bool
		statusFlag         bool
		cleanFlag          bool
		removeFlag         bool
		stopFlag           bool
		downFlag           bool
		freshFlag          bool
		resetDbFlag        bool
		dryRunFlag         bool
		yesFlag            bool
		jsonFlag           bool
		portFlag           bool
		portsFlag          bool
		upgradeFlag        bool
		openFlag           bool
		doctorFlag         bool
		installSkillFlag   bool
		uninstallSkillFlag bool
		refreshSkillFlag   bool
		newFlag            string
		withFlag           string
		completionFlag     string
	)

	flag.BoolVar(&versionFlag, "version", false, "Print version and exit")
	flag.BoolVar(&versionFlag, "v", false, "Print version and exit (shorthand)")

	flag.BoolVar(&listFlag, "list", false, "List all registered projects with their port suffixes")
	flag.BoolVar(&listFlag, "l", false, "List all registered projects (shorthand)")

	flag.BoolVar(&statusFlag, "status", false, "Show status of all registered projects")
	flag.BoolVar(&statusFlag, "s", false, "Show status of all registered projects (shorthand)")

	flag.BoolVar(&cleanFlag, "clean", false, "Remove entries for project directories that no longer exist")
	flag.BoolVar(&cleanFlag, "c", false, "Remove entries for project directories that no longer exist (shorthand)")

	flag.BoolVar(&removeFlag, "remove", false, "Remove the current project from port registry")
	flag.BoolVar(&removeFlag, "r", false, "Remove the current project from port registry (shorthand)")

	flag.BoolVar(&stopFlag, "stop", false, "Run sail stop in the current project")
	flag.BoolVar(&downFlag, "down", false, "Run sail down in the current project")

	flag.BoolVar(&freshFlag, "fresh", false, "Force re-run composer install even if vendor/bin/sail exists")
	flag.BoolVar(&freshFlag, "f", false, "Force re-run composer install (shorthand)")

	flag.BoolVar(&resetDbFlag, "reset-db", false, "Reset database settings to Sail defaults")
	flag.BoolVar(&dryRunFlag, "dry-run", false, "Show what would happen without making changes")
	flag.BoolVar(&dryRunFlag, "d", false, "Show what would happen without making changes (shorthand)")

	flag.BoolVar(&yesFlag, "yes", false, "Automatic yes to prompts; assume yes to all non-interactive prompts")
	flag.BoolVar(&yesFlag, "y", false, "Automatic yes to prompts (shorthand)")
	flag.BoolVar(&yesFlag, "non-interactive", false, "Non-interactive mode")

	flag.BoolVar(&jsonFlag, "json", false, "Output project details in JSON format")
	flag.BoolVar(&jsonFlag, "j", false, "Output project details in JSON format (shorthand)")

	flag.BoolVar(&portFlag, "port", false, "Print calculated APP_PORT for current project and exit")
	flag.BoolVar(&portFlag, "p", false, "Print calculated APP_PORT for current project and exit (shorthand)")

	flag.BoolVar(&portsFlag, "ports", false, "Print every calculated port for the current project and exit")

	flag.StringVar(&newFlag, "new", "", "Create a new Laravel project with the given name")
	flag.StringVar(&newFlag, "n", "", "Create a new Laravel project with the given name (shorthand)")

	flag.StringVar(&withFlag, "with", "mysql", "Services to include when creating a new project (e.g. mysql,redis,mailpit)")
	flag.StringVar(&withFlag, "w", "mysql", "Services to include when creating a new project (shorthand)")

	flag.StringVar(&completionFlag, "completion", "", "Generate shell completion script (bash, zsh, fish)")

	flag.BoolVar(&upgradeFlag, "upgrade", false, "Download and install the latest release over the running binary")
	flag.BoolVar(&upgradeFlag, "u", false, "Download and install the latest release (shorthand)")

	flag.BoolVar(&openFlag, "open", false, "Open the current project's URL in the default browser")
	flag.BoolVar(&openFlag, "o", false, "Open the current project's URL in the default browser (shorthand)")

	flag.BoolVar(&doctorFlag, "doctor", false, "Run diagnostics on the port registry and the current project")

	flag.BoolVar(&installSkillFlag, "install-skill", false, "Install the Claude Code skill for sailinit into ~/.claude/skills")
	flag.BoolVar(&uninstallSkillFlag, "uninstall-skill", false, "Remove the Claude Code skill installed by --install-skill")
	flag.BoolVar(&refreshSkillFlag, "refresh-skill", false, "Update an installed Claude Code skill to this version (run by --upgrade)")

	flag.Parse()

	// Handle --completion flag
	if completionFlag != "" {
		generateCompletion(completionFlag)
		os.Exit(exitOK)
	}

	// Handle --version flag
	if versionFlag {
		fmt.Fprintf(stdout, "sailinit %s\n", version)
		os.Exit(exitOK)
	}

	// Handle --upgrade flag. Runs before any project or Docker checks so it
	// works from any directory.
	if upgradeFlag {
		if err := runUpgrade(dryRunFlag, yesFlag); err != nil {
			printError(fmt.Sprintf("Upgrade failed: %v", err))
			os.Exit(exitError)
		}
		os.Exit(exitOK)
	}

	// Handle the Claude skill flags. Like --upgrade they work from any directory.
	if installSkillFlag || uninstallSkillFlag || refreshSkillFlag {
		var err error
		switch {
		case installSkillFlag:
			err = installSkill(dryRunFlag, yesFlag)
		case uninstallSkillFlag:
			err = uninstallSkill(dryRunFlag, yesFlag)
		default:
			err = refreshSkill(dryRunFlag)
		}
		if errors.Is(err, errSkillAborted) {
			printError("Aborted.")
			os.Exit(exitAborted)
		}
		if err != nil {
			printError(fmt.Sprintf("Claude skill: %v", err))
			os.Exit(exitError)
		}
		os.Exit(exitOK)
	}

	// Handle --port flag
	if portFlag {
		projectDir, err := os.Getwd()
		if err != nil {
			printError(fmt.Sprintf("Error getting current directory: %v", err))
			os.Exit(exitError)
		}
		suggested, existing, existed, err := getSuggestedSuffix(projectDir)
		if err != nil {
			printError(fmt.Sprintf("Error determining port: %v", err))
			os.Exit(exitError)
		}
		ports := CalculatePorts(projectedSuffix(suggested, existing, existed))
		fmt.Fprintln(stdout, ports["APP_PORT"])
		os.Exit(exitOK)
	}

	// Handle --ports flag: every forwarded port for this project, so callers
	// never have to re-derive them from the base offsets (which are
	// configurable, and therefore not safe to hardcode).
	if portsFlag {
		projectDir, err := os.Getwd()
		if err != nil {
			printError(fmt.Sprintf("Error getting current directory: %v", err))
			os.Exit(exitError)
		}
		if err := handlePorts(projectDir, jsonFlag); err != nil {
			printError(fmt.Sprintf("Error determining ports: %v", err))
			os.Exit(exitError)
		}
		os.Exit(exitOK)
	}

	// Handle --doctor flag. Read-only: it reports problems and their fixes but
	// never repairs anything. Exits 1 when a check fails so it can gate a script.
	if doctorFlag {
		projectDir, err := os.Getwd()
		if err != nil {
			printError(fmt.Sprintf("Error getting current directory: %v", err))
			os.Exit(exitError)
		}
		if !runDoctor(projectDir, jsonFlag) {
			os.Exit(exitUnhealthy)
		}
		os.Exit(exitOK)
	}

	// Handle --open flag
	if openFlag {
		projectDir, err := os.Getwd()
		if err != nil {
			printError(fmt.Sprintf("Error getting current directory: %v", err))
			os.Exit(exitError)
		}
		if err := runOpen(projectDir, dryRunFlag); err != nil {
			printError(fmt.Sprintf("Error opening project: %v", err))
			os.Exit(exitError)
		}
		os.Exit(exitOK)
	}

	// Handle --status flag
	if statusFlag {
		if err := showProjectStatus(jsonFlag); err != nil {
			printError(fmt.Sprintf("Error showing status: %v", err))
			os.Exit(exitError)
		}
		os.Exit(exitOK)
	}

	// Handle --list or standalone --json flag
	if listFlag || jsonFlag {
		handleList(jsonFlag)
		os.Exit(exitOK)
	}

	// Handle --clean flag
	if cleanFlag {
		count, err := CleanOrphanedProjects(dryRunFlag)
		if err != nil {
			printError(fmt.Sprintf("Error cleaning orphaned projects: %v", err))
			os.Exit(exitError)
		}
		if dryRunFlag {
			printInfo(fmt.Sprintf("[dry-run] Would clean %d orphaned project(s)", count))
		} else {
			printSuccess(fmt.Sprintf("Cleaned %d orphaned project(s)", count))
		}
		os.Exit(exitOK)
	}

	// Handle --remove flag
	if removeFlag {
		projectDir, err := os.Getwd()
		if err != nil {
			printError(fmt.Sprintf("Error getting current directory: %v", err))
			os.Exit(exitError)
		}
		if err := RemoveProject(projectDir, dryRunFlag); err != nil {
			printError(fmt.Sprintf("Error removing project: %v", err))
			os.Exit(exitError)
		}
		if !dryRunFlag {
			printSuccess("Project removed from port registry.")
		}
		os.Exit(exitOK)
	}

	// Handle --stop flag
	if stopFlag {
		projectDir, err := os.Getwd()
		if err != nil {
			printError(fmt.Sprintf("Error getting current directory: %v", err))
			os.Exit(exitError)
		}
		if err := runSailStop(projectDir); err != nil {
			printError(fmt.Sprintf("Error stopping sail: %v", err))
			os.Exit(exitError)
		}
		os.Exit(exitOK)
	}

	// Handle --down flag
	if downFlag {
		projectDir, err := os.Getwd()
		if err != nil {
			printError(fmt.Sprintf("Error getting current directory: %v", err))
			os.Exit(exitError)
		}
		if err := runSailDown(projectDir); err != nil {
			printError(fmt.Sprintf("Error running sail down: %v", err))
			os.Exit(exitError)
		}
		os.Exit(exitOK)
	}

	// Handle --new flag: create a new Laravel project
	if newFlag != "" {
		projectName := newFlag
		printHeader(fmt.Sprintf("Creating new Laravel project: %s (services: %s)", projectName, withFlag))

		if dryRunFlag {
			printInfo(fmt.Sprintf("[dry-run] Would run: curl -s \"https://laravel.build/%s?with=%s\" | bash", projectName, withFlag))
			printInfo(fmt.Sprintf("[dry-run] Would then set up ports in ./%s", projectName))
			os.Exit(exitOK)
		}

		if err := checkDockerDaemon(); err != nil {
			printError(fmt.Sprintf("Error: %v", err))
			os.Exit(exitError)
		}

		if err := createNewProject(projectName, withFlag); err != nil {
			printError(fmt.Sprintf("Error creating project: %v", err))
			os.Exit(exitError)
		}

		// Change into the new project directory for the rest of the setup
		newDir := filepath.Join(".", projectName)
		absDir, err := filepath.Abs(newDir)
		if err != nil {
			printError(fmt.Sprintf("Error resolving project path: %v", err))
			os.Exit(exitError)
		}
		if err := os.Chdir(absDir); err != nil {
			printError(fmt.Sprintf("Error changing to project directory: %v", err))
			os.Exit(exitError)
		}

		printSuccess(fmt.Sprintf("Project created at %s", absDir))
		printInfo("Configuring ports...")

		// Stop containers started by laravel.build so we can reconfigure ports
		sailPath := filepath.Join(absDir, "vendor", "bin", "sail")
		if _, err := os.Stat(sailPath); err == nil {
			execRunner(sailPath, absDir, "", "down")
		}
	}

	// Main setup flow
	projectDir, err := os.Getwd()
	if err != nil {
		printError(fmt.Sprintf("Error getting current directory: %v", err))
		os.Exit(exitError)
	}

	checkLaravelProject(projectDir, yesFlag)

	detectedVersion := detectPHPVersion(projectDir)
	phpVersion := "84" // Default

	// Check CLI arguments (positional args after flags)
	args := flag.Args()
	if len(args) > 0 {
		phpVersion = args[0]
		if detectedVersion != "" && phpVersion != detectedVersion {
			printWarning(fmt.Sprintf("Warning: Manually specified PHP version (%s) differs from detected version in compose file (%s).", phpVersion, detectedVersion))
			if !confirmOrAbort(
				fmt.Sprintf("PHP %s was requested but the compose file says %s", phpVersion, detectedVersion),
				"re-run with -y to use the requested version, or drop the version argument",
				yesFlag) {
				os.Exit(exitAborted)
			}
		}
	} else if detectedVersion != "" {
		phpVersion = detectedVersion
		printInfo(fmt.Sprintf("Detected PHP version: %s", phpVersion))
	} else {
		printInfo(fmt.Sprintf("No PHP version detected. Using default: %s", phpVersion))
	}

	printHeader(fmt.Sprintf("Starting Laravel Sail setup for PHP %s...", phpVersion))
	suggested, existing, existed, err := getSuggestedSuffix(projectDir)
	if err != nil {
		printError(fmt.Sprintf("Error determining suffix: %v", err))
		os.Exit(exitError)
	}

	if !existed && !existing {
		printInfo("First-ever setup detected.")
		if yesFlag {
			suggested = DefaultStartSuffix
		} else {
			for {
				input := promptLineOrAbort(
					"Enter the starting port suffix for your projects [default 48]: ",
					"no port registry exists yet, so the starting suffix is unset",
					"re-run with -y to start at the default suffix 48")
				if input == "" {
					suggested = DefaultStartSuffix
					break
				}
				var startSuffix int
				_, err := fmt.Sscanf(input, "%d", &startSuffix)
				if err != nil {
					printError("Invalid suffix. Please enter a number.")
					continue
				}
				if err := ValidateSuffix(startSuffix); err != nil {
					printError(fmt.Sprintf("Invalid suffix: %v", err))
					continue
				}
				suggested = startSuffix
				break
			}
		}
	}

	suffix := suggested
	if existing {
		printInfo(fmt.Sprintf("Detected existing port suffix: %d", suffix))
	}

	if !yesFlag {
		for {
			input := promptLineOrAbort(
				fmt.Sprintf("Use suffix [%d]? (Press Enter to confirm, or type new suffix): ", suffix),
				fmt.Sprintf("port suffix %d needs confirming", suffix),
				"re-run with -y to accept the suggested suffix")

			if input != "" {
				var newSuffix int
				_, err := fmt.Sscanf(input, "%d", &newSuffix)
				if err != nil {
					printError("Invalid suffix. Please enter a number.")
					continue
				}
				if err := ValidateSuffix(newSuffix); err != nil {
					printError(fmt.Sprintf("Invalid suffix: %v", err))
					continue
				}
				suffix = newSuffix
			}

			// Validate against collisions
			if otherPath, inUse := isSuffixInUseByOther(projectDir, suffix); inUse {
				printError(fmt.Sprintf("Error: Suffix %d is already in use by another project:\n%s", suffix, otherPath))
				if input == "" {
					suffix = suggested
				}
				continue
			}
			break
		}
	} else {
		if otherPath, inUse := isSuffixInUseByOther(projectDir, suffix); inUse {
			printError(fmt.Sprintf("Error: Suffix %d is already in use by another project:\n%s", suffix, otherPath))
			os.Exit(exitError)
		}
	}

	// Check port availability
	busyPorts := CheckSuffixPortsAvailable(suffix)
	if len(busyPorts) > 0 {
		printWarning("Warning: The following ports are already in use:")
		for _, bp := range busyPorts {
			printWarning(fmt.Sprintf("  %s: %d", bp.Name, bp.Port))
		}
		var names []string
		for _, bp := range busyPorts {
			names = append(names, fmt.Sprintf("%d", bp.Port))
		}
		if !confirmOrAbort(
			"ports "+strings.Join(names, ", ")+" already in use",
			"re-run with -y to accept, or free the ports",
			yesFlag) {
			os.Exit(exitAborted)
		}
	}

	// Save the confirmed suffix
	if dryRunFlag {
		printInfo(fmt.Sprintf("[dry-run] Would save suffix %d for project %s", suffix, projectDir))
	} else {
		if err := saveProjectSuffix(projectDir, suffix); err != nil {
			printError(fmt.Sprintf("Error saving suffix: %v", err))
		}
	}

	printInfo(fmt.Sprintf("Using port suffix: %d", suffix))

	// 1. Setup .env
	if dryRunFlag {
		printInfo(fmt.Sprintf("[dry-run] Would configure .env with suffix %d", suffix))
		printInfo(fmt.Sprintf("[dry-run]   APP_PORT=%d", 8000+suffix))
		printInfo(fmt.Sprintf("[dry-run]   FORWARD_DB_PORT=%d", 3300+suffix))
		printInfo(fmt.Sprintf("[dry-run]   FORWARD_REDIS_PORT=%d", 6300+suffix))
		printInfo(fmt.Sprintf("[dry-run]   FORWARD_MEILISEARCH_PORT=%d", 7700+suffix))
		printInfo(fmt.Sprintf("[dry-run]   FORWARD_MAILPIT_DASHBOARD_PORT=%d", 18100+suffix))
		printInfo(fmt.Sprintf("[dry-run]   FORWARD_MAILPIT_PORT=%d", 1000+suffix))
		printInfo(fmt.Sprintf("[dry-run]   VITE_PORT=%d", 5100+suffix))
	} else {
		if err := setupEnv(projectDir, suffix, resetDbFlag); err != nil {
			printError(fmt.Sprintf("Error setting up .env: %v", err))
			os.Exit(exitError)
		}
	}

	// 2. Initial sailinit logic (Docker composer install)
	if dryRunFlag {
		printInfo(fmt.Sprintf("[dry-run] Would run composer install via Docker (PHP %s)", phpVersion))
	} else {
		if err := runSailInit(phpVersion, projectDir, freshFlag); err != nil {
			printError(fmt.Sprintf("Error running sailinit: %v", err))
			os.Exit(exitError)
		}
	}

	// 3. Run sail up -d
	if dryRunFlag {
		printInfo("[dry-run] Would run sail up -d")
	} else {
		if err := runSailUp(projectDir); err != nil {
			printError(fmt.Sprintf("Error running sail up: %v", err))
			os.Exit(exitError)
		}
	}

	printSuccess("\nSetup complete! Your application is running with the following ports:")
	printInfo(fmt.Sprintf("Main App: http://localhost:%d", 8000+suffix))
	printInfo(fmt.Sprintf("Mailpit Dashboard: http://localhost:%d", 18100+suffix))
}

// projectedSuffix reports the suffix a run would end up using. On a machine
// with no registry yet, setup starts at DefaultStartSuffix rather than at the
// next free slot, so --port and --ports have to say the same thing or they
// would advertise ports that setup never assigns.
func projectedSuffix(suggested int, existing, existed bool) int {
	if !existed && !existing {
		return DefaultStartSuffix
	}
	return suggested
}

// handlePorts prints every port for the project in projectDir. The suffix is
// the one already registered, or the one that would be assigned on the next
// run, so the values are a projection until sailinit has actually run here.
func handlePorts(projectDir string, jsonFormat bool) error {
	suffix, existing, existed, err := getSuggestedSuffix(projectDir)
	if err != nil {
		return err
	}
	suffix = projectedSuffix(suffix, existing, existed)
	ports := CalculatePorts(suffix)

	if jsonFormat {
		out := struct {
			Path   string         `json:"path"`
			Suffix int            `json:"suffix"`
			Ports  map[string]int `json:"ports"`
		}{Path: projectDir, Suffix: suffix, Ports: ports}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(out)
	}

	// Sorted so the output is stable and can be diffed or sourced.
	names := make([]string, 0, len(ports))
	for name := range ports {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(stdout, "%s=%d\n", name, ports[name])
	}
	return nil
}

func handleList(jsonFormat bool) {
	projects, err := ListProjects()
	if err != nil {
		printError(fmt.Sprintf("Error listing projects: %v", err))
		os.Exit(exitError)
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Suffix < projects[j].Suffix
	})

	if jsonFormat {
		type ProjectJSON struct {
			Path       string          `json:"path"`
			Suffix     int             `json:"suffix"`
			Exists     bool            `json:"exists"`
			Ports      map[string]int  `json:"ports"`
			Containers ContainerStatus `json:"containers"`
		}

		var out []ProjectJSON
		for _, p := range projects {
			out = append(out, ProjectJSON{
				Path:       p.Path,
				Suffix:     p.Suffix,
				Exists:     p.Exists,
				Ports:      CalculatePorts(p.Suffix),
				Containers: projectContainerStatus(p),
			})
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		encoder.Encode(out)
		return
	}

	if len(projects) == 0 {
		printInfo("No registered projects found.")
		return
	}

	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		colorize(colorBold, "Project"),
		colorize(colorBold, "Suffix"),
		colorize(colorBold, "App Port"),
		colorize(colorBold, "DB Port"),
		colorize(colorBold, "Redis Port"),
		colorize(colorBold, "Vite Port"),
		colorize(colorBold, "Status"),
	)
	for _, p := range projects {
		status := colorize(colorGreen, "OK")
		if !p.Exists {
			status = colorize(colorRed, "[X] Missing")
		}
		ports := CalculatePorts(p.Suffix)
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%s\n",
			p.Path,
			p.Suffix,
			ports["APP_PORT"],
			ports["FORWARD_DB_PORT"],
			ports["FORWARD_REDIS_PORT"],
			ports["VITE_PORT"],
			status,
		)
	}
	w.Flush()
}

func createNewProject(name, services string) error {
	if _, err := os.Stat(name); err == nil {
		return fmt.Errorf("directory %q already exists", name)
	}
	if services == "" {
		services = "mysql"
	}

	url := fmt.Sprintf("https://laravel.build/%s?with=%s", name, services)
	printInfo(fmt.Sprintf("Downloading from %s ...", url))

	return execRunner("bash", "", fmt.Sprintf(`curl -s "%s" | bash`, url))
}

func runSailInit(phpVersion, projectDir string, forceInstall bool) error {
	sailPath := filepath.Join(projectDir, "vendor", "bin", "sail")
	if !forceInstall {
		if _, err := os.Stat(sailPath); err == nil {
			printInfo("vendor/bin/sail already exists, skipping composer install...")
			return nil
		}
	}

	if err := checkDockerDaemon(); err != nil {
		return err
	}

	printInfo("Installing composer dependencies via Docker (composer:latest)...")

	currentUser := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	dockerImage := "composer:latest"

	return execRunner("docker", "", "", "run", "--rm",
		"-u", currentUser,
		"-v", fmt.Sprintf("%s:/var/www/html", projectDir),
		"-w", "/var/www/html",
		dockerImage,
		"composer", "install", "--ignore-platform-reqs",
	)
}

func setupEnv(projectDir string, suffix int, resetDb bool) error {
	envPath := filepath.Join(projectDir, ".env")
	envExamplePath := filepath.Join(projectDir, ".env.example")

	envCreated := false
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		envCreated = true
		printInfo("Creating .env from .env.example...")
		if _, err := os.Stat(envExamplePath); err == nil {
			data, err := os.ReadFile(envExamplePath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(envPath, data, 0644); err != nil {
				return err
			}
		} else {
			if err := os.WriteFile(envPath, []byte(""), 0644); err != nil {
				return err
			}
		}
	}

	printInfo("Updating .env configuration...")

	data, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}

	lines := splitLines(string(data))

	// Database settings - only apply when .env is newly created or --reset-db flag is used
	applyDbSettings := envCreated || resetDb
	coreUpdates := map[string]string{
		"DB_CONNECTION": "mysql",
		"DB_HOST":       "mysql",
		"DB_PORT":       "3306",
		"DB_DATABASE":   "laravel",
		"DB_USERNAME":   "sail",
		"DB_PASSWORD":   "password",
	}

	portKeys := []string{
		"APP_PORT",
		"FORWARD_DB_PORT",
		"FORWARD_REDIS_PORT",
		"FORWARD_MEILISEARCH_PORT",
		"FORWARD_MAILPIT_DASHBOARD_PORT",
		"FORWARD_MAILPIT_PORT",
		"VITE_PORT",
		"FORWARD_MINIO_PORT",
		"FORWARD_MINIO_CONSOLE_PORT",
		"FORWARD_TYPESENSE_PORT",
		"FORWARD_SOKETI_PORT",
		"FORWARD_SELENIUM_PORT",
	}

	calculatedPorts := CalculatePorts(suffix)
	portValues := map[string]string{
		"APP_PORT":                       fmt.Sprintf("%d", calculatedPorts["APP_PORT"]),
		"FORWARD_DB_PORT":                fmt.Sprintf("%d", calculatedPorts["FORWARD_DB_PORT"]),
		"FORWARD_REDIS_PORT":             fmt.Sprintf("%d", calculatedPorts["FORWARD_REDIS_PORT"]),
		"FORWARD_MEILISEARCH_PORT":       fmt.Sprintf("%d", calculatedPorts["FORWARD_MEILISEARCH_PORT"]),
		"FORWARD_MAILPIT_DASHBOARD_PORT": fmt.Sprintf("%d", calculatedPorts["FORWARD_MAILPIT_DASHBOARD_PORT"]),
		"FORWARD_MAILPIT_PORT":           fmt.Sprintf("%d", calculatedPorts["FORWARD_MAILPIT_PORT"]),
		"VITE_PORT":                      fmt.Sprintf("%d", calculatedPorts["VITE_PORT"]),
		"FORWARD_MINIO_PORT":             fmt.Sprintf("%d", calculatedPorts["FORWARD_MINIO_PORT"]),
		"FORWARD_MINIO_CONSOLE_PORT":     fmt.Sprintf("%d", calculatedPorts["FORWARD_MINIO_CONSOLE_PORT"]),
		"FORWARD_TYPESENSE_PORT":         fmt.Sprintf("%d", calculatedPorts["FORWARD_TYPESENSE_PORT"]),
		"FORWARD_SOKETI_PORT":            fmt.Sprintf("%d", calculatedPorts["FORWARD_SOKETI_PORT"]),
		"FORWARD_SELENIUM_PORT":          fmt.Sprintf("%d", calculatedPorts["FORWARD_SELENIUM_PORT"]),
	}

	var newLines []string
	seen := make(map[string]bool)

	// First pass: update core variables (conditionally) and remove old port/debug variables
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Skip existing port or debug entries
		isSkipKey := false
		for _, pk := range portKeys {
			if strings.HasPrefix(trimmedLine, pk+"=") {
				isSkipKey = true
				break
			}
		}
		if strings.HasPrefix(trimmedLine, "SAIL_XDEBUG_MODE=") {
			isSkipKey = true
		}
		if isSkipKey {
			continue
		}

		// Only update DB settings if applyDbSettings is true
		updated := false
		if applyDbSettings {
			for key, val := range coreUpdates {
				if strings.HasPrefix(trimmedLine, key+"=") {
					newLines = append(newLines, fmt.Sprintf("%s=%s", key, val))
					seen[key] = true
					updated = true
					break
				}
			}
		}
		if !updated {
			newLines = append(newLines, line)
		}
	}

	// Add missing core variables only if applyDbSettings is true
	if applyDbSettings {
		for key, val := range coreUpdates {
			if !seen[key] {
				newLines = append(newLines, fmt.Sprintf("%s=%s", key, val))
			}
		}
	}

	// Remove trailing empty lines
	for len(newLines) > 0 && strings.TrimSpace(newLines[len(newLines)-1]) == "" {
		newLines = newLines[:len(newLines)-1]
	}

	// Final Layout Construction
	newLines = append(newLines, "") // 1. One empty line

	// 2. All port settings together
	newLines = append(newLines, fmt.Sprintf("APP_PORT=%s", portValues["APP_PORT"]))
	newLines = append(newLines, fmt.Sprintf("FORWARD_DB_PORT=%s", portValues["FORWARD_DB_PORT"]))
	newLines = append(newLines, fmt.Sprintf("FORWARD_REDIS_PORT=%s", portValues["FORWARD_REDIS_PORT"]))
	newLines = append(newLines, fmt.Sprintf("FORWARD_MEILISEARCH_PORT=%s", portValues["FORWARD_MEILISEARCH_PORT"]))
	newLines = append(newLines, fmt.Sprintf("FORWARD_MAILPIT_DASHBOARD_PORT=%s", portValues["FORWARD_MAILPIT_DASHBOARD_PORT"]))
	newLines = append(newLines, fmt.Sprintf("FORWARD_MAILPIT_PORT=%s", portValues["FORWARD_MAILPIT_PORT"]))
	newLines = append(newLines, fmt.Sprintf("VITE_PORT=%s", portValues["VITE_PORT"]))
	newLines = append(newLines, fmt.Sprintf("FORWARD_MINIO_PORT=%s", portValues["FORWARD_MINIO_PORT"]))
	newLines = append(newLines, fmt.Sprintf("FORWARD_MINIO_CONSOLE_PORT=%s", portValues["FORWARD_MINIO_CONSOLE_PORT"]))
	newLines = append(newLines, fmt.Sprintf("FORWARD_TYPESENSE_PORT=%s", portValues["FORWARD_TYPESENSE_PORT"]))
	newLines = append(newLines, fmt.Sprintf("FORWARD_SOKETI_PORT=%s", portValues["FORWARD_SOKETI_PORT"]))
	newLines = append(newLines, fmt.Sprintf("FORWARD_SELENIUM_PORT=%s", portValues["FORWARD_SELENIUM_PORT"]))

	newLines = append(newLines, "") // 3. One empty line

	// 4. SAIL_XDEBUG_MODE at the end
	newLines = append(newLines, "SAIL_XDEBUG_MODE=develop,debug,coverage")

	return os.WriteFile(envPath, []byte(strings.Join(newLines, "\n")+"\n"), 0644)
}

func runSailUp(projectDir string) error {
	sailPath := filepath.Join(projectDir, "vendor", "bin", "sail")
	if _, err := os.Stat(sailPath); os.IsNotExist(err) {
		return fmt.Errorf("sail binary not found at %s", sailPath)
	}

	if err := checkDockerDaemon(); err != nil {
		return err
	}

	printInfo("Starting Laravel Sail (sail up -d)...")
	return execRunner(sailPath, projectDir, "", "up", "-d")
}

func runSailStop(projectDir string) error {
	sailPath := filepath.Join(projectDir, "vendor", "bin", "sail")
	if _, err := os.Stat(sailPath); os.IsNotExist(err) {
		return fmt.Errorf("sail binary not found at %s", sailPath)
	}

	if err := checkDockerDaemon(); err != nil {
		return err
	}

	printInfo("Stopping Laravel Sail...")
	return execRunner(sailPath, projectDir, "", "stop")
}

func runSailDown(projectDir string) error {
	sailPath := filepath.Join(projectDir, "vendor", "bin", "sail")
	if _, err := os.Stat(sailPath); os.IsNotExist(err) {
		return fmt.Errorf("sail binary not found at %s", sailPath)
	}

	if err := checkDockerDaemon(); err != nil {
		return err
	}

	printInfo("Running sail down...")
	return execRunner(sailPath, projectDir, "", "down")
}

// ContainerStatus describes a project's containers. State is a stable
// machine-readable token; Running is how many are up. Keeping the count and the
// rendering apart is what lets the JSON output stay free of ANSI codes.
type ContainerStatus struct {
	State   string `json:"state"`
	Running int    `json:"running"`
}

// Container states.
const (
	stateRunning = "running"
	stateStopped = "stopped"
	stateNoSail  = "no sail"
	stateUnknown = "unknown"
	stateMissing = "missing"
)

// Running reports whether the project has live containers.
func (c ContainerStatus) IsRunning() bool {
	return c.Running > 0
}

// Display renders the status for a terminal table.
func (c ContainerStatus) Display() string {
	switch c.State {
	case stateRunning:
		return colorize(colorGreen, fmt.Sprintf("%d running", c.Running))
	case stateStopped:
		return colorize(colorDim, stateStopped)
	case stateMissing:
		return colorize(colorRed, "[X] Missing")
	default:
		return c.State
	}
}

func getContainerStatus(projectDir string) ContainerStatus {
	sailPath := filepath.Join(projectDir, "vendor", "bin", "sail")
	if _, err := os.Stat(sailPath); os.IsNotExist(err) {
		return ContainerStatus{State: stateNoSail}
	}

	output, err := execOutputRunner(sailPath, projectDir, "ps", "--format", "{{.State}}")
	if err != nil {
		return ContainerStatus{State: stateUnknown}
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	running := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == stateRunning {
			running++
		}
	}

	if running == 0 {
		return ContainerStatus{State: stateStopped}
	}
	return ContainerStatus{State: stateRunning, Running: running}
}

// projectContainerStatus reports the container state for a registered project,
// without shelling out for one whose directory is gone.
func projectContainerStatus(p ProjectInfo) ContainerStatus {
	if !p.Exists {
		return ContainerStatus{State: stateMissing}
	}
	return getContainerStatus(p.Path)
}

func showProjectStatus(jsonFormat bool) error {
	projects, err := ListProjects()
	if err != nil {
		return err
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Suffix < projects[j].Suffix
	})

	if jsonFormat {
		type StatusJSON struct {
			Path       string          `json:"path"`
			Suffix     int             `json:"suffix"`
			Exists     bool            `json:"exists"`
			AppPort    int             `json:"app_port"`
			Containers ContainerStatus `json:"containers"`
		}

		out := make([]StatusJSON, 0, len(projects))
		for _, p := range projects {
			out = append(out, StatusJSON{
				Path:       p.Path,
				Suffix:     p.Suffix,
				Exists:     p.Exists,
				AppPort:    CalculatePorts(p.Suffix)["APP_PORT"],
				Containers: projectContainerStatus(p),
			})
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(out)
	}

	if len(projects) == 0 {
		printInfo("No registered projects found.")
		return nil
	}

	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
		colorize(colorBold, "Project"),
		colorize(colorBold, "Suffix"),
		colorize(colorBold, "App Port"),
		colorize(colorBold, "Containers"),
	)
	for _, p := range projects {
		containers := projectContainerStatus(p).Display()
		ports := CalculatePorts(p.Suffix)
		fmt.Fprintf(w, "%s\t%d\t%d\t%s\n",
			p.Path,
			p.Suffix,
			ports["APP_PORT"],
			containers,
		)
	}
	w.Flush()
	return nil
}
