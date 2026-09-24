package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// skillContent is the Claude Code skill shipped with this build. It is
// embedded rather than downloaded so the installed skill always describes the
// flags of the binary that wrote it.
//
//go:embed skill/SKILL.md
var skillContent []byte

// skillName is the directory the skill lives in under <claude>/skills.
const skillName = "sailinit"

// errSkillAborted means the user declined to overwrite or delete a file
// sailinit does not own.
var errSkillAborted = errors.New("aborted")

// skillState records what --install-skill wrote. The hash is what lets a
// refresh tell "our file, safe to replace" from "the user edited it".
type skillState struct {
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Version string `json:"version"`
}

// runningUnderSudo reports whether sailinit runs as root on behalf of another
// user. It is a variable so tests can simulate it.
var runningUnderSudo = func() bool {
	return os.Geteuid() == 0 && os.Getenv("SUDO_USER") != ""
}

// claudeConfigDir returns Claude Code's config directory, honouring
// CLAUDE_CONFIG_DIR the same way Claude Code does.
func claudeConfigDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

func skillFilePath() (string, error) {
	dir, err := claudeConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "skills", skillName, "SKILL.md"), nil
}

// skillStatePath keeps the tracking file next to the registry, so
// SAILINIT_HOME isolates it too. It is separate from config.json because that
// file is user-edited and sailinit never writes it.
func skillStatePath() (string, error) {
	if home := sailinitHome(); home != "" {
		return filepath.Join(home, "skill.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "sailinit", "skill.json"), nil
}

// loadSkillState returns nil, nil when no skill has been installed.
func loadSkillState() (*skillState, error) {
	path, err := skillStatePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var st skillState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if st.Path == "" {
		return nil, nil
	}
	return &st, nil
}

func saveSkillState(st *skillState) error {
	path, err := skillStatePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func removeSkillState() error {
	path, err := skillStatePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// writeSkill writes the embedded skill to path and records it as ours.
func writeSkill(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, skillContent, 0644); err != nil {
		return err
	}
	return saveSkillState(&skillState{Path: path, SHA256: sha256Hex(skillContent), Version: version})
}

// ownedBy reports whether data at path is exactly what st says sailinit wrote.
func (st *skillState) ownedBy(path string, data []byte) bool {
	return st != nil && st.Path == path && st.SHA256 == sha256Hex(data)
}

// installSkill writes the skill into Claude Code's skills directory. A file
// sailinit did not write, or one edited since, is only replaced after
// confirmation.
func installSkill(dryRun, assumeYes bool) error {
	path, err := skillFilePath()
	if err != nil {
		return fmt.Errorf("locating the Claude config directory: %w", err)
	}
	state, err := loadSkillState()
	if err != nil {
		return err
	}

	existing, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		// Nothing there yet.
	case err != nil:
		return err
	case bytes.Equal(existing, skillContent):
		if dryRun {
			printInfo(fmt.Sprintf("[dry-run] %s is already up to date", path))
			return nil
		}
		if err := saveSkillState(&skillState{Path: path, SHA256: sha256Hex(skillContent), Version: version}); err != nil {
			return err
		}
		printSuccess(fmt.Sprintf("Claude skill already up to date at %s", path))
		return nil
	case !state.ownedBy(path, existing):
		if dryRun {
			printInfo(fmt.Sprintf("[dry-run] Would overwrite %s, which sailinit did not write or which has local edits (needs confirmation)", path))
			return nil
		}
		printWarning(fmt.Sprintf("Warning: %s already exists and was not written by this sailinit, or has local edits.", path))
		if !confirmOrAbort(
			"overwrite "+path,
			"re-run with -y to replace it, or move the file aside first",
			assumeYes) {
			return errSkillAborted
		}
	}

	if dryRun {
		printInfo(fmt.Sprintf("[dry-run] Would write the Claude skill to %s", path))
		return nil
	}
	if err := writeSkill(path); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Installed Claude skill at %s", path))
	printInfo("sailinit --upgrade will keep it up to date.")
	return nil
}

// refreshSkill rewrites an installed skill with this build's version. It never
// installs one that is not tracked, never recreates one the user deleted, and
// never overwrites local edits.
func refreshSkill(dryRun bool) error {
	state, err := loadSkillState()
	if err != nil {
		return err
	}
	if state == nil {
		printInfo("No Claude skill installed by sailinit; nothing to refresh. Run sailinit --install-skill to add it.")
		return nil
	}

	existing, err := os.ReadFile(state.Path)
	if os.IsNotExist(err) {
		printInfo(fmt.Sprintf("Claude skill at %s was removed; not recreating it. Run sailinit --install-skill to restore it, or --uninstall-skill to stop tracking it.", state.Path))
		return nil
	}
	if err != nil {
		return err
	}
	if !state.ownedBy(state.Path, existing) {
		printWarning(fmt.Sprintf("Claude skill at %s has local edits; not overwriting. Run sailinit --install-skill to replace it.", state.Path))
		return nil
	}
	if bytes.Equal(existing, skillContent) {
		if dryRun {
			printInfo(fmt.Sprintf("[dry-run] Claude skill at %s is already up to date", state.Path))
			return nil
		}
		state.Version = version
		if err := saveSkillState(state); err != nil {
			return err
		}
		printSuccess(fmt.Sprintf("Claude skill already up to date at %s", state.Path))
		return nil
	}

	if dryRun {
		printInfo(fmt.Sprintf("[dry-run] Would update the Claude skill at %s", state.Path))
		return nil
	}
	if err := writeSkill(state.Path); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Updated Claude skill at %s", state.Path))
	return nil
}

// uninstallSkill removes the skill sailinit installed and stops tracking it.
// It only ever deletes the tracked file, and asks first if it was edited.
func uninstallSkill(dryRun, assumeYes bool) error {
	state, err := loadSkillState()
	if err != nil {
		return err
	}
	if state == nil {
		printInfo("No Claude skill installed by sailinit; nothing to remove.")
		return nil
	}

	existing, err := os.ReadFile(state.Path)
	switch {
	case os.IsNotExist(err):
		// Already gone; only the tracking entry is left.
	case err != nil:
		return err
	case !state.ownedBy(state.Path, existing):
		if dryRun {
			printInfo(fmt.Sprintf("[dry-run] Would remove %s, which has local edits (needs confirmation)", state.Path))
			return nil
		}
		printWarning(fmt.Sprintf("Warning: %s has local edits.", state.Path))
		if !confirmOrAbort(
			"delete the edited skill at "+state.Path,
			"re-run with -y to delete it anyway",
			assumeYes) {
			return errSkillAborted
		}
	}

	if dryRun {
		printInfo(fmt.Sprintf("[dry-run] Would remove the Claude skill at %s", state.Path))
		return nil
	}
	if err := os.Remove(state.Path); err != nil && !os.IsNotExist(err) {
		return err
	}
	// Only succeeds when the directory is empty, so anything the user added
	// alongside the skill is left alone.
	os.Remove(filepath.Dir(state.Path))
	if err := removeSkillState(); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Removed Claude skill from %s", state.Path))
	return nil
}

// refreshSkillAfterUpgrade runs the freshly installed binary's --refresh-skill.
// It has to be the new binary: this process still carries the old skill.
func refreshSkillAfterUpgrade(exePath string) {
	// Under sudo the home directory, and so both the skill and its tracking
	// file, may be root's; writing there would leave root-owned files behind.
	if runningUnderSudo() {
		printInfo("If you installed the Claude skill, run 'sailinit --refresh-skill' without sudo to update it.")
		return
	}
	state, err := loadSkillState()
	if err != nil || state == nil {
		return
	}
	if err := execRunner(exePath, "", "", "--refresh-skill"); err != nil {
		printWarning(fmt.Sprintf("Upgrade succeeded, but refreshing the Claude skill failed: %v", err))
	}
}

// checkSkill reports on the Claude skill for --doctor. Every problem is a
// WARN: the skill is optional and never affects the exit code.
func checkSkill() Diagnostic {
	const name = "Claude skill"
	state, err := loadSkillState()
	if err != nil {
		return warn(name, fmt.Sprintf("tracking file is unreadable: %v", err), "run sailinit --install-skill to rewrite it")
	}
	if state == nil {
		if path, err := skillFilePath(); err == nil && fileExists(path) {
			return ok(name, fmt.Sprintf("present at %s but not managed by sailinit", path))
		}
		return ok(name, "not installed (optional: sailinit --install-skill)")
	}

	existing, err := os.ReadFile(state.Path)
	if os.IsNotExist(err) {
		return warn(name, fmt.Sprintf("was removed from %s", state.Path),
			"run sailinit --install-skill to restore it, or sailinit --uninstall-skill to stop tracking it")
	}
	if err != nil {
		return warn(name, fmt.Sprintf("cannot read %s: %v", state.Path, err), "")
	}
	if !state.ownedBy(state.Path, existing) {
		return warn(name, fmt.Sprintf("%s has local edits, so upgrades will not refresh it", state.Path),
			"run sailinit --install-skill to replace it with this version")
	}
	if !bytes.Equal(existing, skillContent) {
		return warn(name, fmt.Sprintf("%s is from an older sailinit (%s)", state.Path, state.Version),
			"run sailinit --refresh-skill")
	}
	return ok(name, fmt.Sprintf("up to date at %s", state.Path))
}
