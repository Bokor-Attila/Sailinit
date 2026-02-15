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
)

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

func main() {
	listFlag := flag.Bool("list", false, "List all registered projects with their port suffixes")
	cleanFlag := flag.Bool("clean", false, "Remove entries for project directories that no longer exist")
	freshFlag := flag.Bool("fresh", false, "Force re-run composer install even if vendor/bin/sail exists")
	resetDbFlag := flag.Bool("reset-db", false, "Reset database settings to Sail defaults (mysql, laravel, sail/password)")
	flag.Parse()

	// Handle --list flag
	if *listFlag {
		projects, err := ListProjects()
		if err != nil {
			fmt.Printf("Error listing projects: %v\n", err)
			os.Exit(1)
		}
		if len(projects) == 0 {
			fmt.Println("No registered projects found.")
			os.Exit(0)
		}

		// Sort by suffix for consistent output
		sort.Slice(projects, func(i, j int) bool {
			return projects[i].Suffix < projects[j].Suffix
		})

		fmt.Printf("     %-40s %-8s %s\n", "Project", "Suffix", "App Port")
		for _, p := range projects {
			prefix := "    "
			if !p.Exists {
				prefix = "[X]"
			}
			fmt.Printf("%s  %-40s %-8d %d\n", prefix, p.Path, p.Suffix, 8000+p.Suffix)
		}
		os.Exit(0)
	}

	// Handle --clean flag
	if *cleanFlag {
		count, err := CleanOrphanedProjects()
		if err != nil {
			fmt.Printf("Error cleaning orphaned projects: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Cleaned %d orphaned project(s)\n", count)
		os.Exit(0)
	}

	projectDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	detectedVersion := detectPHPVersion(projectDir)
	phpVersion := "84" // Default

	// Check CLI arguments (positional args after flags)
	args := flag.Args()
	if len(args) > 0 {
		phpVersion = args[0]
		if detectedVersion != "" && phpVersion != detectedVersion {
			fmt.Printf("Warning: Manually specified PHP version (%s) differs from detected version in compose file (%s).\n", phpVersion, detectedVersion)
			fmt.Print("Continue anyway? [y/N]: ")
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				os.Exit(0)
			}
		}
	} else if detectedVersion != "" {
		phpVersion = detectedVersion
		fmt.Printf("Detected PHP version: %s\n", phpVersion)
	} else {
		fmt.Printf("No PHP version detected. Using default: %s\n", phpVersion)
	}

	fmt.Printf("Starting Laravel Sail setup for PHP %s...\n", phpVersion)
	suggested, existing, existed, err := getSuggestedSuffix(projectDir)
	if err != nil {
		fmt.Printf("Error determining suffix: %v\n", err)
		os.Exit(1)
	}

	reader := bufio.NewReader(os.Stdin)
	if !existed && !existing {
		fmt.Println("First-ever setup detected.")
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
				fmt.Println("Invalid suffix. Please enter a number.")
				continue
			}
			suggested = startSuffix
			break
		}
	}

	suffix := suggested
	if existing {
		fmt.Printf("Detected existing port suffix: %d\n", suffix)
	}

	for {
		fmt.Printf("Use suffix [%d]? (Press Enter to confirm, or type new suffix): ", suffix)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input != "" {
			var newSuffix int
			_, err := fmt.Sscanf(input, "%d", &newSuffix)
			if err != nil {
				fmt.Println("Invalid suffix. Please enter a number.")
				continue
			}
			suffix = newSuffix
		}

		// Validate against collisions
		if otherPath, inUse := isSuffixInUseByOther(projectDir, suffix); inUse {
			fmt.Printf("Error: Suffix %d is already in use by another project:\n%s\n", suffix, otherPath)
			// Reset suffix to suggested and retry loop but only if user didn't enter it
			if input == "" {
				// This shouldn't happen if the suggested one is correct, but for safety:
				suffix = suggested
			}
			continue
		}
		break
	}

	// Save the confirmed suffix
	if err := saveProjectSuffix(projectDir, suffix); err != nil {
		fmt.Printf("Error saving suffix: %v\n", err)
	}

	fmt.Printf("Using port suffix: %d\n", suffix)

	// 1. Setup .env
	if err := setupEnv(projectDir, suffix, *resetDbFlag); err != nil {
		fmt.Printf("Error setting up .env: %v\n", err)
		os.Exit(1)
	}

	// 2. Initial sailinit logic (Docker composer install)
	if err := runSailInit(phpVersion, projectDir, *freshFlag); err != nil {
		fmt.Printf("Error running sailinit: %v\n", err)
		os.Exit(1)
	}

	// 3. Run sail up -d
	if err := runSailUp(projectDir); err != nil {
		fmt.Printf("Error running sail up: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nSetup complete! Your application is running with the following ports:")
	fmt.Printf("Main App: http://localhost:%d\n", 8000+suffix)
	fmt.Printf("Mailpit Dashboard: http://localhost:%d\n", 18100+suffix)
}

func runSailInit(phpVersion, projectDir string, forceInstall bool) error {
	sailPath := filepath.Join(projectDir, "vendor", "bin", "sail")
	if !forceInstall {
		if _, err := os.Stat(sailPath); err == nil {
			fmt.Println("vendor/bin/sail already exists, skipping composer install...")
			return nil
		}
	}

	fmt.Println("Installing composer dependencies via Docker...")

	currentUser := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	dockerImage := fmt.Sprintf("laravelsail/php%s-composer:latest", phpVersion)

	cmd := exec.Command("docker", "run", "--rm",
		"-u", currentUser,
		"-v", fmt.Sprintf("%s:/var/www/html", projectDir),
		"-w", "/var/www/html",
		dockerImage,
		"composer", "install", "--ignore-platform-reqs",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func setupEnv(projectDir string, suffix int, resetDb bool) error {
	envPath := filepath.Join(projectDir, ".env")
	envExamplePath := filepath.Join(projectDir, ".env.example")

	envCreated := false
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		envCreated = true
		fmt.Println("Creating .env from .env.example...")
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

	fmt.Println("Updating .env configuration...")

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
	}

	portValues := map[string]string{
		"APP_PORT":                       fmt.Sprintf("%d", 8000+suffix),
		"FORWARD_DB_PORT":                fmt.Sprintf("%d", 3300+suffix),
		"FORWARD_REDIS_PORT":             fmt.Sprintf("%d", 6300+suffix),
		"FORWARD_MEILISEARCH_PORT":       fmt.Sprintf("%d", 7700+suffix),
		"FORWARD_MAILPIT_DASHBOARD_PORT": fmt.Sprintf("%d", 18100+suffix),
		"FORWARD_MAILPIT_PORT":           fmt.Sprintf("%d", 1000+suffix),
		"VITE_PORT":                      fmt.Sprintf("%d", 5100+suffix),
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

	newLines = append(newLines, "") // 3. One empty line

	// 4. SAIL_XDEBUG_MODE at the end
	newLines = append(newLines, "SAIL_XDEBUG_MODE=develop,debug,coverage")

	return os.WriteFile(envPath, []byte(strings.Join(newLines, "\n")+"\n"), 0644)
}

func runSailUp(projectDir string) error {
	fmt.Println("Starting Laravel Sail (sail up -d)...")

	sailPath := filepath.Join(projectDir, "vendor", "bin", "sail")
	if _, err := os.Stat(sailPath); os.IsNotExist(err) {
		return fmt.Errorf("sail binary not found at %s", sailPath)
	}

	cmd := exec.Command(sailPath, "up", "-d")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
