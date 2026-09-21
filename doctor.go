package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Diagnostic severity levels. Only statusFail sets a non-zero exit code, so
// scripts can treat sailinit --doctor as a pass/fail gate.
const (
	statusOK   = "OK"
	statusWarn = "WARN"
	statusFail = "FAIL"
)

// Diagnostic is a single check result.
type Diagnostic struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	// Fix is the suggested remedy, empty when there is nothing to do.
	Fix string `json:"fix,omitempty"`
}

func ok(name, detail string) Diagnostic {
	return Diagnostic{Name: name, Status: statusOK, Detail: detail}
}

func warn(name, detail, fix string) Diagnostic {
	return Diagnostic{Name: name, Status: statusWarn, Detail: detail, Fix: fix}
}

func fail(name, detail, fix string) Diagnostic {
	return Diagnostic{Name: name, Status: statusFail, Detail: detail, Fix: fix}
}

// checkDocker reports whether the Docker daemon is reachable.
func checkDocker() Diagnostic {
	if err := checkDockerDaemon(); err != nil {
		return warn("Docker", "daemon is not reachable", "start Docker Desktop or the Docker service")
	}
	return ok("Docker", "daemon is running")
}

// checkRegistry reports on the port state file itself.
func checkRegistry() []Diagnostic {
	path, err := getPortStatePath()
	if err != nil {
		return []Diagnostic{fail("Registry", fmt.Sprintf("cannot determine state file path: %v", err), "")}
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []Diagnostic{warn("Registry", fmt.Sprintf("no state file yet at %s", path), "run sailinit in a project to create one")}
	}

	state, _, err := loadPortState()
	if err != nil {
		return []Diagnostic{fail("Registry", fmt.Sprintf("state file at %s is unreadable: %v", path, err), "repair or delete the file, then re-register projects")}
	}

	diags := []Diagnostic{ok("Registry", fmt.Sprintf("%d project(s) tracked in %s", len(state.Projects), path))}

	// MaxSuffix must cover every allocated suffix, or the next allocation
	// collides with an existing project.
	highest := 0
	for _, s := range state.Projects {
		if s > highest {
			highest = s
		}
	}
	if highest > state.MaxSuffix {
		diags = append(diags, fail("Registry max suffix",
			fmt.Sprintf("max_suffix is %d but a project uses %d; the next project would collide", state.MaxSuffix, highest),
			"run sailinit --clean, or correct max_suffix in the state file"))
	}

	return diags
}

// checkDuplicateSuffixes finds projects sharing a suffix, which means they
// share every forwarded port.
func checkDuplicateSuffixes() Diagnostic {
	state, _, err := loadPortState()
	if err != nil {
		return warn("Duplicate suffixes", "could not read the registry", "")
	}

	bySuffix := make(map[int][]string)
	for dir, suffix := range state.Projects {
		bySuffix[suffix] = append(bySuffix[suffix], dir)
	}

	var clashes []string
	for suffix, dirs := range bySuffix {
		if len(dirs) > 1 {
			sort.Strings(dirs)
			clashes = append(clashes, fmt.Sprintf("suffix %d: %s", suffix, strings.Join(dirs, ", ")))
		}
	}

	if len(clashes) == 0 {
		return ok("Duplicate suffixes", "every project has a unique suffix")
	}

	sort.Strings(clashes)
	return fail("Duplicate suffixes", strings.Join(clashes, "; "),
		"run sailinit --remove in one project, then sailinit to reassign it")
}

// checkOrphans finds registry rows whose directory no longer exists.
func checkOrphans() Diagnostic {
	projects, err := ListProjects()
	if err != nil {
		return warn("Orphaned projects", "could not read the registry", "")
	}

	var missing []string
	for _, p := range projects {
		if !p.Exists {
			missing = append(missing, p.Path)
		}
	}

	if len(missing) == 0 {
		return ok("Orphaned projects", "every registered directory exists")
	}

	sort.Strings(missing)
	return warn("Orphaned projects",
		fmt.Sprintf("%d registered director(ies) no longer exist: %s", len(missing), strings.Join(missing, ", ")),
		"run sailinit --clean")
}

// checkCurrentProject inspects the directory sailinit was invoked from.
// Returns nil when the current directory is not a Laravel project, so the
// command stays useful when run from anywhere.
func checkCurrentProject(projectDir string) []Diagnostic {
	hasComposer := fileExists(filepath.Join(projectDir, "composer.json"))
	hasArtisan := fileExists(filepath.Join(projectDir, "artisan"))
	if !hasComposer && !hasArtisan {
		return nil
	}

	var diags []Diagnostic

	state, _, err := loadPortState()
	if err != nil {
		return []Diagnostic{warn("Current project", "could not read the registry", "")}
	}

	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return []Diagnostic{warn("Current project", fmt.Sprintf("could not resolve %s", projectDir), "")}
	}

	registered, isRegistered := state.Projects[absDir]
	if !isRegistered {
		diags = append(diags, warn("Current project",
			"this Laravel project is not registered yet",
			"run sailinit here to assign it a port suffix"))
		return diags
	}

	ports := CalculatePorts(registered)
	diags = append(diags, ok("Current project",
		fmt.Sprintf("registered with suffix %d (APP_PORT %d)", registered, ports["APP_PORT"])))

	// .env drift: the registry and the file the containers actually read
	// must agree, or sail publishes ports nothing else knows about.
	envPath := filepath.Join(projectDir, ".env")
	if fileExists(envPath) {
		envSuffix, found := extractSuffixFromEnv(envPath)
		switch {
		case !found:
			diags = append(diags, warn(".env ports",
				"no APP_PORT found in .env",
				"run sailinit here to write the port block"))
		case envSuffix != registered:
			envPorts := CalculatePorts(envSuffix)
			diags = append(diags, fail(".env ports",
				fmt.Sprintf(".env uses APP_PORT %d (suffix %d) but the registry says APP_PORT %d (suffix %d)",
					envPorts["APP_PORT"], envSuffix, ports["APP_PORT"], registered),
				"run sailinit here to rewrite .env from the registry"))
		default:
			diags = append(diags, ok(".env ports", fmt.Sprintf(".env matches the registry (APP_PORT %d)", ports["APP_PORT"])))
		}
	} else {
		diags = append(diags, warn(".env ports", "no .env file in this project", "run sailinit here to create one"))
	}

	// Busy ports are only a warning: another process may legitimately hold
	// them, and sail will surface the real failure on startup.
	if busy := CheckSuffixPortsAvailable(registered); len(busy) > 0 {
		var names []string
		for _, b := range busy {
			names = append(names, fmt.Sprintf("%s (%d)", b.Name, b.Port))
		}
		diags = append(diags, warn("Port availability",
			fmt.Sprintf("%d port(s) already in use: %s", len(busy), strings.Join(names, ", ")),
			"stop whatever holds them, or reassign this project with sailinit --remove then sailinit"))
	} else {
		diags = append(diags, ok("Port availability", "all forwarded ports are free"))
	}

	return diags
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// runDoctor executes every diagnostic and reports them. It returns true when
// no check failed, which the caller turns into the process exit code.
func runDoctor(projectDir string, jsonFormat bool) bool {
	var diags []Diagnostic
	diags = append(diags, checkDocker())
	diags = append(diags, checkRegistry()...)
	diags = append(diags, checkDuplicateSuffixes())
	diags = append(diags, checkOrphans())
	diags = append(diags, checkCurrentProject(projectDir)...)

	healthy := true
	for _, d := range diags {
		if d.Status == statusFail {
			healthy = false
		}
	}

	if jsonFormat {
		out := struct {
			Healthy     bool         `json:"healthy"`
			Diagnostics []Diagnostic `json:"diagnostics"`
		}{Healthy: healthy, Diagnostics: diags}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			printError(fmt.Sprintf("Error encoding diagnostics: %v", err))
			return false
		}
		fmt.Fprintln(stdout, string(data))
		return healthy
	}

	// The report is this command's output, so all of it goes to stdout rather
	// than through the print* helpers, which write to stderr.
	fmt.Fprintln(stdout, colorize(colorBold, "sailinit doctor"))
	for _, d := range diags {
		var label string
		switch d.Status {
		case statusOK:
			label = colorize(colorGreen, "[ OK ]")
		case statusWarn:
			label = colorize(colorYellow, "[WARN]")
		default:
			label = colorize(colorRed, "[FAIL]")
		}
		fmt.Fprintf(stdout, "%s %s: %s\n", label, d.Name, d.Detail)
		if d.Fix != "" && d.Status != statusOK {
			fmt.Fprintf(stdout, "       %s\n", colorize(colorDim, "fix: "+d.Fix))
		}
	}

	fmt.Fprintln(stdout)
	if healthy {
		fmt.Fprintln(stdout, colorize(colorGreen, "No problems found."))
	} else {
		fmt.Fprintln(stdout, colorize(colorRed, "Problems found. See the suggested fixes above."))
	}
	return healthy
}
