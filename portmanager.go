package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MaxPortSuffix is the highest valid suffix (65535 - 18100, the highest base port)
const MaxPortSuffix = 47435

// DefaultStartSuffix is the suffix the very first project gets when no registry
// exists yet. --port and --ports project it too, so what they report matches
// what a subsequent run would actually assign.
const DefaultStartSuffix = 48

// ValidateSuffix checks that a port suffix is within valid range.
func ValidateSuffix(suffix int) error {
	if suffix < 0 {
		return fmt.Errorf("suffix must be non-negative, got %d", suffix)
	}
	if suffix > MaxPortSuffix {
		return fmt.Errorf("suffix %d too large: highest port would be %d (max 65535)", suffix, 18100+suffix)
	}
	return nil
}

// CheckPortAvailable returns true if the given TCP port is not in use.
func CheckPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// BusyPort holds info about an unavailable port.
type BusyPort struct {
	Name string
	Port int
}

type GlobalConfig struct {
	BaseAppPort              int `json:"base_app_port"`
	BaseDbPort               int `json:"base_db_port"`
	BaseRedisPort            int `json:"base_redis_port"`
	BaseMeilisearchPort      int `json:"base_meilisearch_port"`
	BaseMailpitDashboardPort int `json:"base_mailpit_dashboard_port"`
	BaseMailpitPort          int `json:"base_mailpit_port"`
	BaseVitePort             int `json:"base_vite_port"`
	BaseMinioPort            int `json:"base_minio_port"`
	BaseMinioConsolePort     int `json:"base_minio_console_port"`
	BaseTypesensePort        int `json:"base_typesense_port"`
	BaseSoketiPort           int `json:"base_soketi_port"`
	BaseSeleniumPort         int `json:"base_selenium_port"`
}

func DefaultGlobalConfig() GlobalConfig {
	return GlobalConfig{
		BaseAppPort:              8000,
		BaseDbPort:               3300,
		BaseRedisPort:            6300,
		BaseMeilisearchPort:      7700,
		BaseMailpitDashboardPort: 18100,
		BaseMailpitPort:          1000,
		BaseVitePort:             5100,
		BaseMinioPort:            9000,
		BaseMinioConsolePort:     8900,
		BaseTypesensePort:        8108,
		BaseSoketiPort:           6001,
		BaseSeleniumPort:         4444,
	}
}

// sailinitHome returns the directory named by SAILINIT_HOME, or "" when it is
// unset. When it is set, sailinit keeps both the registry and the global config
// there and ignores the legacy home-directory files, so a sandboxed or scripted
// run cannot pick up (or write to) the real ones.
func sailinitHome() string {
	return strings.TrimSpace(os.Getenv("SAILINIT_HOME"))
}

func getGlobalConfigPath() string {
	if home := sailinitHome(); home != "" {
		return filepath.Join(home, "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	legacy := filepath.Join(home, ".sailinit-config.json")
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "sailinit", "config.json")
}

func loadGlobalConfig() GlobalConfig {
	cfg := DefaultGlobalConfig()
	path := getGlobalConfigPath()
	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			var loaded GlobalConfig
			if err := json.Unmarshal(data, &loaded); err == nil {
				if loaded.BaseAppPort > 0 {
					cfg.BaseAppPort = loaded.BaseAppPort
				}
				if loaded.BaseDbPort > 0 {
					cfg.BaseDbPort = loaded.BaseDbPort
				}
				if loaded.BaseRedisPort > 0 {
					cfg.BaseRedisPort = loaded.BaseRedisPort
				}
				if loaded.BaseMeilisearchPort > 0 {
					cfg.BaseMeilisearchPort = loaded.BaseMeilisearchPort
				}
				if loaded.BaseMailpitDashboardPort > 0 {
					cfg.BaseMailpitDashboardPort = loaded.BaseMailpitDashboardPort
				}
				if loaded.BaseMailpitPort > 0 {
					cfg.BaseMailpitPort = loaded.BaseMailpitPort
				}
				if loaded.BaseVitePort > 0 {
					cfg.BaseVitePort = loaded.BaseVitePort
				}
				if loaded.BaseMinioPort > 0 {
					cfg.BaseMinioPort = loaded.BaseMinioPort
				}
				if loaded.BaseMinioConsolePort > 0 {
					cfg.BaseMinioConsolePort = loaded.BaseMinioConsolePort
				}
				if loaded.BaseTypesensePort > 0 {
					cfg.BaseTypesensePort = loaded.BaseTypesensePort
				}
				if loaded.BaseSoketiPort > 0 {
					cfg.BaseSoketiPort = loaded.BaseSoketiPort
				}
				if loaded.BaseSeleniumPort > 0 {
					cfg.BaseSeleniumPort = loaded.BaseSeleniumPort
				}
			}
		}
	}

	parseEnv := func(envKey string, target *int) {
		if val := os.Getenv(envKey); val != "" {
			var p int
			if _, err := fmt.Sscanf(val, "%d", &p); err == nil && p > 0 {
				*target = p
			}
		}
	}
	parseEnv("SAILINIT_BASE_APP_PORT", &cfg.BaseAppPort)
	parseEnv("SAILINIT_BASE_DB_PORT", &cfg.BaseDbPort)
	parseEnv("SAILINIT_BASE_REDIS_PORT", &cfg.BaseRedisPort)
	parseEnv("SAILINIT_BASE_MEILISEARCH_PORT", &cfg.BaseMeilisearchPort)
	parseEnv("SAILINIT_BASE_MAILPIT_DASHBOARD_PORT", &cfg.BaseMailpitDashboardPort)
	parseEnv("SAILINIT_BASE_MAILPIT_PORT", &cfg.BaseMailpitPort)
	parseEnv("SAILINIT_BASE_VITE_PORT", &cfg.BaseVitePort)
	parseEnv("SAILINIT_BASE_MINIO_PORT", &cfg.BaseMinioPort)
	parseEnv("SAILINIT_BASE_MINIO_CONSOLE_PORT", &cfg.BaseMinioConsolePort)
	parseEnv("SAILINIT_BASE_TYPESENSE_PORT", &cfg.BaseTypesensePort)
	parseEnv("SAILINIT_BASE_SOKETI_PORT", &cfg.BaseSoketiPort)
	parseEnv("SAILINIT_BASE_SELENIUM_PORT", &cfg.BaseSeleniumPort)

	return cfg
}

func CalculatePorts(suffix int) map[string]int {
	cfg := loadGlobalConfig()
	return map[string]int{
		"APP_PORT":                       cfg.BaseAppPort + suffix,
		"FORWARD_DB_PORT":                cfg.BaseDbPort + suffix,
		"FORWARD_REDIS_PORT":             cfg.BaseRedisPort + suffix,
		"FORWARD_MEILISEARCH_PORT":       cfg.BaseMeilisearchPort + suffix,
		"FORWARD_MAILPIT_DASHBOARD_PORT": cfg.BaseMailpitDashboardPort + suffix,
		"FORWARD_MAILPIT_PORT":           cfg.BaseMailpitPort + suffix,
		"VITE_PORT":                      cfg.BaseVitePort + suffix,
		"FORWARD_MINIO_PORT":             cfg.BaseMinioPort + suffix,
		"FORWARD_MINIO_CONSOLE_PORT":     cfg.BaseMinioConsolePort + suffix,
		"FORWARD_TYPESENSE_PORT":         cfg.BaseTypesensePort + suffix,
		"FORWARD_SOKETI_PORT":            cfg.BaseSoketiPort + suffix,
		"FORWARD_SELENIUM_PORT":          cfg.BaseSeleniumPort + suffix,
	}
}

// CheckSuffixPortsAvailable checks ports for a suffix and returns busy ones.
func CheckSuffixPortsAvailable(suffix int) []BusyPort {
	calculated := CalculatePorts(suffix)
	ports := []struct {
		name string
		port int
	}{
		{"APP_PORT", calculated["APP_PORT"]},
		{"FORWARD_DB_PORT", calculated["FORWARD_DB_PORT"]},
		{"FORWARD_REDIS_PORT", calculated["FORWARD_REDIS_PORT"]},
		{"FORWARD_MEILISEARCH_PORT", calculated["FORWARD_MEILISEARCH_PORT"]},
		{"FORWARD_MAILPIT_DASHBOARD_PORT", calculated["FORWARD_MAILPIT_DASHBOARD_PORT"]},
		{"FORWARD_MAILPIT_PORT", calculated["FORWARD_MAILPIT_PORT"]},
		{"VITE_PORT", calculated["VITE_PORT"]},
		{"FORWARD_MINIO_PORT", calculated["FORWARD_MINIO_PORT"]},
		{"FORWARD_MINIO_CONSOLE_PORT", calculated["FORWARD_MINIO_CONSOLE_PORT"]},
		{"FORWARD_TYPESENSE_PORT", calculated["FORWARD_TYPESENSE_PORT"]},
		{"FORWARD_SOKETI_PORT", calculated["FORWARD_SOKETI_PORT"]},
		{"FORWARD_SELENIUM_PORT", calculated["FORWARD_SELENIUM_PORT"]},
	}

	var busy []BusyPort
	for _, p := range ports {
		if !CheckPortAvailable(p.port) {
			busy = append(busy, BusyPort{Name: p.name, Port: p.port})
		}
	}
	return busy
}

// RemoveProject removes a project from the port state file. With dryRun it
// reports what it would remove and leaves the registry untouched.
func RemoveProject(projectDir string, dryRun bool) error {
	state, _, err := loadPortState()
	if err != nil {
		return err
	}

	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return err
	}

	suffix, ok := state.Projects[absDir]
	if !ok {
		return fmt.Errorf("project not registered: %s", absDir)
	}

	if dryRun {
		printInfo(fmt.Sprintf("[dry-run] Would remove %s (suffix %d) from the registry", absDir, suffix))
		return nil
	}

	delete(state.Projects, absDir)
	return state.save()
}

type PortState struct {
	MaxSuffix int            `json:"max_suffix"`
	Projects  map[string]int `json:"projects"`
}

type ProjectInfo struct {
	Path   string
	Suffix int
	Exists bool
}

// testStatePathOverride is used only for testing to override the state file path
var testStatePathOverride string

func getPortStatePath() (string, error) {
	if testStatePathOverride != "" {
		return testStatePathOverride, nil
	}

	// SAILINIT_HOME wins over every discovered location, including the legacy
	// file, so an isolated registry stays isolated.
	if dir := sailinitHome(); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", err
		}
		return filepath.Join(dir, "ports.json"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// 1. Backward compatibility: if legacy state file exists in home, keep using it!
	legacyPath := filepath.Join(home, ".laravel-sail-ports.json")
	if _, err := os.Stat(legacyPath); err == nil {
		return legacyPath, nil
	}

	// 2. Modern XDG Config directory for new installations
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(home, ".config")
	}
	targetDir := filepath.Join(configDir, "sailinit")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return legacyPath, nil
	}
	return filepath.Join(targetDir, "ports.json"), nil
}

func loadPortState() (*PortState, bool, error) {
	path, err := getPortStatePath()
	if err != nil {
		return nil, false, err
	}

	state := &PortState{
		MaxSuffix: 0,
		Projects:  make(map[string]int),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return state, false, nil
		}
		return nil, false, err
	}

	if err := json.Unmarshal(data, state); err != nil {
		return nil, false, err
	}

	return state, true, nil
}

func (s *PortState) save() error {
	path, err := getPortStatePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(dir, "ports-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}

	return os.Rename(tmpName, path)
}

func getSuggestedSuffix(projectDir string) (int, bool, bool, error) {
	state, existed, err := loadPortState()
	if err != nil {
		return 0, false, false, err
	}

	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return 0, false, false, err
	}

	// 1. Try to find in state by project directory
	if suffix, ok := state.Projects[absDir]; ok {
		return suffix, true, existed, nil
	}

	// 2. Try to find in .env if it exists
	envPath := filepath.Join(projectDir, ".env")
	if _, err := os.Stat(envPath); err == nil {
		suffix, found := extractSuffixFromEnv(envPath)
		if found {
			return suffix, true, existed, nil
		}
	}

	// 3. Suggest new allocation
	return state.MaxSuffix + 1, false, existed, nil
}

func saveProjectSuffix(projectDir string, suffix int) error {
	state, _, err := loadPortState()
	if err != nil {
		return err
	}

	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return err
	}

	state.Projects[absDir] = suffix
	if suffix > state.MaxSuffix {
		state.MaxSuffix = suffix
	}

	return state.save()
}

func isSuffixInUseByOther(projectDir string, suffix int) (string, bool) {
	state, _, err := loadPortState()
	if err != nil {
		return "", false
	}

	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return "", false
	}

	for path, s := range state.Projects {
		if s == suffix && path != absDir {
			return path, true
		}
	}

	return "", false
}

func extractSuffixFromEnv(envPath string) (int, bool) {
	data, err := os.ReadFile(envPath)
	if err != nil {
		return 0, false
	}

	content := string(data)
	lines := splitLines(content)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "APP_PORT=") {
			var p int
			_, err := fmt.Sscanf(line, "APP_PORT=%d", &p)
			if err == nil && p >= 8000 {
				return p - 8000, true
			}
		}
	}

	return 0, false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			end := i
			if end > start && s[end-1] == '\r' {
				end--
			}
			lines = append(lines, s[start:end])
			start = i + 1
		}
	}
	if start < len(s) {
		line := s[start:]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		lines = append(lines, line)
	}
	return lines
}

func ListProjects() ([]ProjectInfo, error) {
	state, _, err := loadPortState()
	if err != nil {
		return nil, err
	}

	var projects []ProjectInfo
	for path, suffix := range state.Projects {
		exists := true
		if _, err := os.Stat(path); os.IsNotExist(err) {
			exists = false
		}
		projects = append(projects, ProjectInfo{
			Path:   path,
			Suffix: suffix,
			Exists: exists,
		})
	}

	return projects, nil
}

// CleanOrphanedProjects drops registry entries whose directory no longer
// exists. With dryRun it reports what it would drop and writes nothing.
func CleanOrphanedProjects(dryRun bool) (int, error) {
	state, _, err := loadPortState()
	if err != nil {
		return 0, err
	}

	var removed []string
	for path := range state.Projects {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			removed = append(removed, path)
		}
	}

	// Sorted so a dry-run preview and the real run list the same thing in the
	// same order.
	sort.Strings(removed)

	for _, path := range removed {
		if dryRun {
			printInfo(fmt.Sprintf("[dry-run] Would remove orphaned project: %s (suffix %d)", path, state.Projects[path]))
			continue
		}
		printInfo(fmt.Sprintf("Removing orphaned project: %s (suffix %d)", path, state.Projects[path]))
		delete(state.Projects, path)
	}

	if dryRun {
		return len(removed), nil
	}

	if len(removed) > 0 {
		if err := state.save(); err != nil {
			return 0, err
		}
	}

	return len(removed), nil
}
