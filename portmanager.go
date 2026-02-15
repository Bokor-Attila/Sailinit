package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".laravel-sail-ports.json"), nil
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

	return os.WriteFile(path, data, 0644)
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

func CleanOrphanedProjects() (int, error) {
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

	for _, path := range removed {
		fmt.Printf("Removing orphaned project: %s (suffix %d)\n", path, state.Projects[path])
		delete(state.Projects, path)
	}

	if len(removed) > 0 {
		if err := state.save(); err != nil {
			return 0, err
		}
	}

	return len(removed), nil
}
