package main

import (
	"bufio"
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

func generateCompletion(shell string) {
	switch strings.ToLower(shell) {
	case "bash":
		fmt.Print(`# bash completion for sailinit
_sailinit() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    opts="--version -v --list -l --status -s --clean -c --remove -r --stop --down --fresh -f --reset-db --dry-run -d --yes -y --non-interactive --new -n --with -w --completion"

    if [[ ${cur} == -* ]] ; then
        COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
        return 0
    fi
}
complete -F _sailinit sailinit
`)
	case "zsh":
		fmt.Print(`# zsh completion for sailinit
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
        '--completion[Generate shell completion script]:shell:(bash zsh fish)'
    )
    _describe -t commands 'sailinit flags' options
}

_sailinit "$@"
`)
	case "fish":
		fmt.Print(`# fish completion for sailinit
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
complete -c sailinit -l completion -r -f -a 'bash zsh fish' -d 'Generate shell completion script'
`)
	default:
		printError(fmt.Sprintf("Unknown shell: %s. Supported shells: bash, zsh, fish", shell))
		os.Exit(1)
	}
}

func main() {
	var (
		versionFlag    bool
		listFlag       bool
		statusFlag     bool
		cleanFlag      bool
		removeFlag     bool
		stopFlag       bool
		downFlag       bool
		freshFlag      bool
		resetDbFlag    bool
		dryRunFlag     bool
		yesFlag        bool
		newFlag        string
		withFlag       string
		completionFlag string
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

	flag.StringVar(&newFlag, "new", "", "Create a new Laravel project with the given name")
	flag.StringVar(&newFlag, "n", "", "Create a new Laravel project with the given name (shorthand)")

	flag.StringVar(&withFlag, "with", "mysql", "Services to include when creating a new project (e.g. mysql,redis,mailpit)")
	flag.StringVar(&withFlag, "w", "mysql", "Services to include when creating a new project (shorthand)")

	flag.StringVar(&completionFlag, "completion", "", "Generate shell completion script (bash, zsh, fish)")

	flag.Parse()

	// Handle --completion flag
	if completionFlag != "" {
		generateCompletion(completionFlag)
		os.Exit(0)
	}

	// Handle --version flag
	if versionFlag {
		fmt.Printf("sailinit %s\n", version)
		os.Exit(0)
	}

	// Handle --list flag
	if listFlag {
		handleList()
		os.Exit(0)
	}

	// Handle --status flag
	if statusFlag {
		if err := showProjectStatus(); err != nil {
			printError(fmt.Sprintf("Error showing status: %v", err))
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --clean flag
	if cleanFlag {
		count, err := CleanOrphanedProjects()
		if err != nil {
			printError(fmt.Sprintf("Error cleaning orphaned projects: %v", err))
			os.Exit(1)
		}
		printSuccess(fmt.Sprintf("Cleaned %d orphaned project(s)", count))
		os.Exit(0)
	}

	// Handle --remove flag
	if removeFlag {
		projectDir, err := os.Getwd()
		if err != nil {
			printError(fmt.Sprintf("Error getting current directory: %v", err))
			os.Exit(1)
		}
		if err := RemoveProject(projectDir); err != nil {
			printError(fmt.Sprintf("Error removing project: %v", err))
			os.Exit(1)
		}
		printSuccess("Project removed from port registry.")
		os.Exit(0)
	}

	// Handle --stop flag
	if stopFlag {
		projectDir, err := os.Getwd()
		if err != nil {
			printError(fmt.Sprintf("Error getting current directory: %v", err))
			os.Exit(1)
		}
		if err := runSailStop(projectDir); err != nil {
			printError(fmt.Sprintf("Error stopping sail: %v", err))
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --down flag
	if downFlag {
		projectDir, err := os.Getwd()
		if err != nil {
			printError(fmt.Sprintf("Error getting current directory: %v", err))
			os.Exit(1)
		}
		if err := runSailDown(projectDir); err != nil {
			printError(fmt.Sprintf("Error running sail down: %v", err))
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --new flag: create a new Laravel project
	if newFlag != "" {
		projectName := newFlag
		printHeader(fmt.Sprintf("Creating new Laravel project: %s (services: %s)", projectName, withFlag))

		if dryRunFlag {
			printInfo(fmt.Sprintf("[dry-run] Would run: curl -s \"https://laravel.build/%s?with=%s\" | bash", projectName, withFlag))
			printInfo(fmt.Sprintf("[dry-run] Would then set up ports in ./%s", projectName))
			os.Exit(0)
		}

		if err := checkDockerDaemon(); err != nil {
			printError(fmt.Sprintf("Error: %v", err))
			os.Exit(1)
		}

		if err := createNewProject(projectName, withFlag); err != nil {
			printError(fmt.Sprintf("Error creating project: %v", err))
			os.Exit(1)
		}

		// Change into the new project directory for the rest of the setup
		newDir := filepath.Join(".", projectName)
		absDir, err := filepath.Abs(newDir)
		if err != nil {
			printError(fmt.Sprintf("Error resolving project path: %v", err))
			os.Exit(1)
		}
		if err := os.Chdir(absDir); err != nil {
			printError(fmt.Sprintf("Error changing to project directory: %v", err))
			os.Exit(1)
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
		os.Exit(1)
	}

	detectedVersion := detectPHPVersion(projectDir)
	phpVersion := "84" // Default

	// Check CLI arguments (positional args after flags)
	args := flag.Args()
	if len(args) > 0 {
		phpVersion = args[0]
		if detectedVersion != "" && phpVersion != detectedVersion {
			printWarning(fmt.Sprintf("Warning: Manually specified PHP version (%s) differs from detected version in compose file (%s).", phpVersion, detectedVersion))
			if !yesFlag {
				fmt.Print("Continue anyway? [y/N]: ")
				var confirm string
				fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					os.Exit(0)
				}
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
		os.Exit(1)
	}

	reader := bufio.NewReader(os.Stdin)
	if !existed && !existing {
		printInfo("First-ever setup detected.")
		if yesFlag {
			suggested = 48
		} else {
			for {
				fmt.Print("Enter the starting port suffix for your projects [default 48]: ")
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(input)
				if input == "" {
					suggested = 48
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
			fmt.Printf("Use suffix [%d]? (Press Enter to confirm, or type new suffix): ", suffix)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)

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
			os.Exit(1)
		}
	}

	// Check port availability
	busyPorts := CheckSuffixPortsAvailable(suffix)
	if len(busyPorts) > 0 {
		printWarning("Warning: The following ports are already in use:")
		for _, bp := range busyPorts {
			printWarning(fmt.Sprintf("  %s: %d", bp.Name, bp.Port))
		}
		if !yesFlag {
			fmt.Print("Continue anyway? [y/N]: ")
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				os.Exit(0)
			}
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
			os.Exit(1)
		}
	}

	// 2. Initial sailinit logic (Docker composer install)
	if dryRunFlag {
		printInfo(fmt.Sprintf("[dry-run] Would run composer install via Docker (PHP %s)", phpVersion))
	} else {
		if err := runSailInit(phpVersion, projectDir, freshFlag); err != nil {
			printError(fmt.Sprintf("Error running sailinit: %v", err))
			os.Exit(1)
		}
	}

	// 3. Run sail up -d
	if dryRunFlag {
		printInfo("[dry-run] Would run sail up -d")
	} else {
		if err := runSailUp(projectDir); err != nil {
			printError(fmt.Sprintf("Error running sail up: %v", err))
			os.Exit(1)
		}
	}

	printSuccess("\nSetup complete! Your application is running with the following ports:")
	printInfo(fmt.Sprintf("Main App: http://localhost:%d", 8000+suffix))
	printInfo(fmt.Sprintf("Mailpit Dashboard: http://localhost:%d", 18100+suffix))
}

func handleList() {
	projects, err := ListProjects()
	if err != nil {
		printError(fmt.Sprintf("Error listing projects: %v", err))
		os.Exit(1)
	}
	if len(projects) == 0 {
		printInfo("No registered projects found.")
		return
	}

	// Sort by suffix for consistent output
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Suffix < projects[j].Suffix
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
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
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%s\n",
			p.Path,
			p.Suffix,
			8000+p.Suffix,
			3300+p.Suffix,
			6300+p.Suffix,
			5100+p.Suffix,
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

	portValues := map[string]string{
		"APP_PORT":                       fmt.Sprintf("%d", 8000+suffix),
		"FORWARD_DB_PORT":                fmt.Sprintf("%d", 3300+suffix),
		"FORWARD_REDIS_PORT":             fmt.Sprintf("%d", 6300+suffix),
		"FORWARD_MEILISEARCH_PORT":       fmt.Sprintf("%d", 7700+suffix),
		"FORWARD_MAILPIT_DASHBOARD_PORT": fmt.Sprintf("%d", 18100+suffix),
		"FORWARD_MAILPIT_PORT":           fmt.Sprintf("%d", 1000+suffix),
		"VITE_PORT":                      fmt.Sprintf("%d", 5100+suffix),
		"FORWARD_MINIO_PORT":             fmt.Sprintf("%d", 9000+suffix),
		"FORWARD_MINIO_CONSOLE_PORT":     fmt.Sprintf("%d", 8900+suffix),
		"FORWARD_TYPESENSE_PORT":         fmt.Sprintf("%d", 8108+suffix),
		"FORWARD_SOKETI_PORT":            fmt.Sprintf("%d", 6001+suffix),
		"FORWARD_SELENIUM_PORT":          fmt.Sprintf("%d", 4444+suffix),
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

func getContainerStatus(projectDir string) string {
	sailPath := filepath.Join(projectDir, "vendor", "bin", "sail")
	if _, err := os.Stat(sailPath); os.IsNotExist(err) {
		return "no sail"
	}

	output, err := execOutputRunner(sailPath, projectDir, "ps", "--format", "{{.State}}")
	if err != nil {
		return "unknown"
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	running := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "running" {
			running++
		}
	}

	if running == 0 {
		return colorize(colorDim, "stopped")
	}
	return colorize(colorGreen, fmt.Sprintf("%d running", running))
}

func showProjectStatus() error {
	projects, err := ListProjects()
	if err != nil {
		return err
	}
	if len(projects) == 0 {
		printInfo("No registered projects found.")
		return nil
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Suffix < projects[j].Suffix
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
		colorize(colorBold, "Project"),
		colorize(colorBold, "Suffix"),
		colorize(colorBold, "App Port"),
		colorize(colorBold, "Containers"),
	)
	for _, p := range projects {
		containers := colorize(colorRed, "[X] Missing")
		if p.Exists {
			containers = getContainerStatus(p.Path)
		}
		fmt.Fprintf(w, "%s\t%d\t%d\t%s\n",
			p.Path,
			p.Suffix,
			8000+p.Suffix,
			containers,
		)
	}
	w.Flush()
	return nil
}
