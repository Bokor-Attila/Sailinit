package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// skillSandbox points both the Claude config dir and SAILINIT_HOME at temp
// dirs and returns where the skill file would be written.
func skillSandbox(t *testing.T) (skillPath, sailinitHomeDir string) {
	t.Helper()
	claudeDir := t.TempDir()
	sailinitHomeDir = t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("SAILINIT_HOME", sailinitHomeDir)

	origStderr := stderr
	stderr = &bytes.Buffer{}
	t.Cleanup(func() { stderr = origStderr })

	return filepath.Join(claudeDir, "skills", "sailinit", "SKILL.md"), sailinitHomeDir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// simulateOlderInstall makes the installed skill look like one an older
// sailinit wrote and tracked: different content, matching hash.
func simulateOlderInstall(t *testing.T, path string) {
	t.Helper()
	old := "old skill\n"
	writeFile(t, path, old)
	if err := saveSkillState(&skillState{Path: path, SHA256: sha256Hex([]byte(old)), Version: "v2.0.0"}); err != nil {
		t.Fatal(err)
	}
}

func TestInstallSkillWritesAndTracks(t *testing.T) {
	path, _ := skillSandbox(t)

	if err := installSkill(false, false); err != nil {
		t.Fatalf("installSkill: %v", err)
	}
	if got := readFile(t, path); got != string(skillContent) {
		t.Error("installed skill does not match the embedded one")
	}
	st, err := loadSkillState()
	if err != nil || st == nil {
		t.Fatalf("expected tracking state, got %v, %v", st, err)
	}
	if st.Path != path || st.SHA256 != sha256Hex(skillContent) {
		t.Errorf("state = %+v, want path %s and the embedded hash", st, path)
	}
}

func TestInstallSkillDryRunWritesNothing(t *testing.T) {
	path, home := skillSandbox(t)

	if err := installSkill(true, false); err != nil {
		t.Fatalf("installSkill dry-run: %v", err)
	}
	if fileExists(path) {
		t.Error("dry-run wrote the skill file")
	}
	if fileExists(filepath.Join(home, "skill.json")) {
		t.Error("dry-run wrote the tracking file")
	}
}

func TestInstallSkillRefusesForeignFileWithoutConsent(t *testing.T) {
	path, _ := skillSandbox(t)
	writeFile(t, path, "hand-written skill\n")

	origInteractive := isInteractive
	isInteractive = func() bool { return true }
	origInput, origReader := promptInput, promptReader
	promptInput, promptReader = strings.NewReader("n\n"), nil
	t.Cleanup(func() {
		isInteractive = origInteractive
		promptInput, promptReader = origInput, origReader
	})

	err := installSkill(false, false)
	if !errors.Is(err, errSkillAborted) {
		t.Fatalf("expected errSkillAborted, got %v", err)
	}
	if got := readFile(t, path); got != "hand-written skill\n" {
		t.Error("a declined install overwrote the existing file")
	}
}

func TestInstallSkillOverwritesForeignFileWithYes(t *testing.T) {
	path, _ := skillSandbox(t)
	writeFile(t, path, "hand-written skill\n")

	if err := installSkill(false, true); err != nil {
		t.Fatalf("installSkill -y: %v", err)
	}
	if got := readFile(t, path); got != string(skillContent) {
		t.Error("-y did not replace the existing file")
	}
}

func TestInstallSkillUpdatesOwnOlderFileWithoutPrompting(t *testing.T) {
	path, _ := skillSandbox(t)
	simulateOlderInstall(t, path)

	origInteractive := isInteractive
	isInteractive = func() bool { t.Fatal("prompted for a file sailinit owns"); return false }
	t.Cleanup(func() { isInteractive = origInteractive })

	if err := installSkill(false, false); err != nil {
		t.Fatalf("installSkill: %v", err)
	}
	if got := readFile(t, path); got != string(skillContent) {
		t.Error("own older skill was not updated")
	}
}

func TestRefreshSkillUpdatesTrackedUnmodifiedSkill(t *testing.T) {
	path, _ := skillSandbox(t)
	simulateOlderInstall(t, path)

	if err := refreshSkill(false); err != nil {
		t.Fatalf("refreshSkill: %v", err)
	}
	if got := readFile(t, path); got != string(skillContent) {
		t.Error("refresh did not update a tracked, unmodified skill")
	}
	st, _ := loadSkillState()
	if st == nil || st.SHA256 != sha256Hex(skillContent) {
		t.Errorf("state not updated after refresh: %+v", st)
	}
}

func TestRefreshSkillSkipsWhenNeverInstalled(t *testing.T) {
	path, _ := skillSandbox(t)

	if err := refreshSkill(false); err != nil {
		t.Fatalf("refreshSkill: %v", err)
	}
	if fileExists(path) {
		t.Error("refresh installed a skill that was never installed")
	}
}

func TestRefreshSkillDoesNotRecreateDeletedSkill(t *testing.T) {
	path, _ := skillSandbox(t)
	simulateOlderInstall(t, path)
	os.Remove(path)

	if err := refreshSkill(false); err != nil {
		t.Fatalf("refreshSkill: %v", err)
	}
	if fileExists(path) {
		t.Error("refresh recreated a skill the user deleted")
	}
}

func TestRefreshSkillKeepsLocalEdits(t *testing.T) {
	path, _ := skillSandbox(t)
	simulateOlderInstall(t, path)
	writeFile(t, path, "old skill\nmy own notes\n")

	if err := refreshSkill(false); err != nil {
		t.Fatalf("refreshSkill: %v", err)
	}
	if got := readFile(t, path); got != "old skill\nmy own notes\n" {
		t.Error("refresh overwrote a locally edited skill")
	}
}

func TestRefreshSkillDryRunChangesNothing(t *testing.T) {
	path, _ := skillSandbox(t)
	simulateOlderInstall(t, path)

	if err := refreshSkill(true); err != nil {
		t.Fatalf("refreshSkill dry-run: %v", err)
	}
	if got := readFile(t, path); got != "old skill\n" {
		t.Error("dry-run refresh changed the skill")
	}
}

func TestUninstallSkillRemovesFileAndTracking(t *testing.T) {
	path, home := skillSandbox(t)
	if err := installSkill(false, false); err != nil {
		t.Fatal(err)
	}

	if err := uninstallSkill(false, false); err != nil {
		t.Fatalf("uninstallSkill: %v", err)
	}
	if fileExists(path) {
		t.Error("skill file still present")
	}
	if fileExists(filepath.Dir(path)) {
		t.Error("empty skill directory still present")
	}
	if fileExists(filepath.Join(home, "skill.json")) {
		t.Error("tracking file still present")
	}
}

func TestUninstallSkillLeavesUntrackedFileAlone(t *testing.T) {
	path, _ := skillSandbox(t)
	writeFile(t, path, "hand-written skill\n")

	if err := uninstallSkill(false, true); err != nil {
		t.Fatalf("uninstallSkill: %v", err)
	}
	if !fileExists(path) {
		t.Error("uninstall deleted a skill sailinit never installed")
	}
}

func TestUninstallSkillEditedNeedsConsent(t *testing.T) {
	path, _ := skillSandbox(t)
	simulateOlderInstall(t, path)
	writeFile(t, path, "edited\n")

	origInteractive := isInteractive
	isInteractive = func() bool { return true }
	origInput, origReader := promptInput, promptReader
	promptInput, promptReader = strings.NewReader("n\n"), nil
	t.Cleanup(func() {
		isInteractive = origInteractive
		promptInput, promptReader = origInput, origReader
	})

	if err := uninstallSkill(false, false); !errors.Is(err, errSkillAborted) {
		t.Fatalf("expected errSkillAborted, got %v", err)
	}
	if !fileExists(path) {
		t.Error("declined uninstall deleted the edited skill")
	}
}

func TestCheckSkill(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(t *testing.T, path string)
		status string
		detail string
	}{
		{"not installed", func(t *testing.T, path string) {}, statusOK, "not installed"},
		{"foreign file", func(t *testing.T, path string) { writeFile(t, path, "x") }, statusOK, "not managed"},
		{"up to date", func(t *testing.T, path string) {
			if err := installSkill(false, false); err != nil {
				t.Fatal(err)
			}
		}, statusOK, "up to date"},
		{"stale", simulateOlderInstall, statusWarn, "older sailinit"},
		{"edited", func(t *testing.T, path string) {
			simulateOlderInstall(t, path)
			writeFile(t, path, "edited")
		}, statusWarn, "local edits"},
		{"removed", func(t *testing.T, path string) {
			simulateOlderInstall(t, path)
			os.Remove(path)
		}, statusWarn, "was removed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, _ := skillSandbox(t)
			tt.setup(t, path)
			d := checkSkill()
			if d.Status != tt.status || !strings.Contains(d.Detail, tt.detail) {
				t.Errorf("checkSkill() = %s %q, want %s containing %q", d.Status, d.Detail, tt.status, tt.detail)
			}
		})
	}
}

// recordRunner swaps execRunner for one that records invocations.
func recordRunner(t *testing.T) *[]string {
	t.Helper()
	var calls []string
	orig := execRunner
	execRunner = func(name string, dir string, stdin string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return nil
	}
	t.Cleanup(func() { execRunner = orig })
	return &calls
}

func withSudo(t *testing.T, sudo bool) {
	t.Helper()
	orig := runningUnderSudo
	runningUnderSudo = func() bool { return sudo }
	t.Cleanup(func() { runningUnderSudo = orig })
}

func TestUpgradeRefreshesTrackedSkillWithNewBinary(t *testing.T) {
	srv := fakeRelease{tag: "v9.9.9", assetBody: []byte("new binary"), withChecksum: true}.start(t)
	exe := installFixture(t, "old binary")
	withUpgradeEnv(t, srv.URL, "v1.4.0", exe, "9.9.9")
	path, _ := skillSandbox(t)
	simulateOlderInstall(t, path)
	withSudo(t, false)
	calls := recordRunner(t)

	if err := runUpgrade(false, false); err != nil {
		t.Fatalf("runUpgrade: %v", err)
	}
	if len(*calls) != 1 || (*calls)[0] != exe+" --refresh-skill" {
		t.Errorf("calls = %v, want exactly [%s --refresh-skill]", *calls, exe)
	}
}

func TestUpgradeSkipsSkillRefreshWhenNotInstalled(t *testing.T) {
	srv := fakeRelease{tag: "v9.9.9", assetBody: []byte("new binary"), withChecksum: true}.start(t)
	exe := installFixture(t, "old binary")
	withUpgradeEnv(t, srv.URL, "v1.4.0", exe, "9.9.9")
	skillSandbox(t)
	withSudo(t, false)
	calls := recordRunner(t)

	if err := runUpgrade(false, false); err != nil {
		t.Fatalf("runUpgrade: %v", err)
	}
	if len(*calls) != 0 {
		t.Errorf("refresh ran for an untracked skill: %v", *calls)
	}
}

func TestUpgradeSkipsSkillRefreshUnderSudo(t *testing.T) {
	srv := fakeRelease{tag: "v9.9.9", assetBody: []byte("new binary"), withChecksum: true}.start(t)
	exe := installFixture(t, "old binary")
	withUpgradeEnv(t, srv.URL, "v1.4.0", exe, "9.9.9")
	path, _ := skillSandbox(t)
	simulateOlderInstall(t, path)
	withSudo(t, true)
	calls := recordRunner(t)

	if err := runUpgrade(false, false); err != nil {
		t.Fatalf("runUpgrade: %v", err)
	}
	if len(*calls) != 0 {
		t.Errorf("refresh ran under sudo: %v", *calls)
	}
	if !strings.Contains(stderr.(*bytes.Buffer).String(), "--refresh-skill") {
		t.Error("sudo upgrade did not tell the user how to refresh the skill")
	}
}

// skillFlagExclusions are flags the skill deliberately does not mention.
var skillFlagExclusions = map[string]string{
	"non-interactive": "alias of --yes",
	"completion":      "shell setup, not useful to an agent",
}

// TestSkillMentionsEveryFlag keeps the skill from drifting behind the CLI: a
// new flag must be documented in skill/SKILL.md or excluded above.
func TestSkillMentionsEveryFlag(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`flag\.(?:Bool|String)Var\(&\w+, "([a-z][a-z-]+)"`)
	matches := re.FindAllStringSubmatch(string(src), -1)
	if len(matches) < 10 {
		t.Fatalf("found only %d flags in main.go; the pattern is out of date", len(matches))
	}
	for _, m := range matches {
		name := m[1]
		if _, skip := skillFlagExclusions[name]; skip {
			continue
		}
		mentioned := regexp.MustCompile(`--` + regexp.QuoteMeta(name) + `($|[^a-z-])`)
		if !mentioned.Match(skillContent) {
			t.Errorf("skill/SKILL.md does not mention --%s; document it or add it to skillFlagExclusions", name)
		}
	}
}

func TestCLIInstallSkillEndToEnd(t *testing.T) {
	home, project := sandbox(t)
	claudeDir := t.TempDir()
	envs := append(env(home), "CLAUDE_CONFIG_DIR="+claudeDir)
	path := filepath.Join(claudeDir, "skills", "sailinit", "SKILL.md")

	if res := run(t, project, envs, "--install-skill"); res.code != exitOK {
		t.Fatalf("--install-skill exit %d, stderr: %s", res.code, res.stderr)
	}
	if got := readFile(t, path); got != string(skillContent) {
		t.Error("CLI install wrote the wrong content")
	}
	if res := run(t, project, envs, "--uninstall-skill"); res.code != exitOK {
		t.Fatalf("--uninstall-skill exit %d, stderr: %s", res.code, res.stderr)
	}
	if fileExists(path) {
		t.Error("CLI uninstall left the skill behind")
	}
}

func TestCLIInstallSkillOverForeignFileWithoutTTYAborts(t *testing.T) {
	home, project := sandbox(t)
	claudeDir := t.TempDir()
	path := filepath.Join(claudeDir, "skills", "sailinit", "SKILL.md")
	writeFile(t, path, "hand-written skill\n")

	res := run(t, project, append(env(home), "CLAUDE_CONFIG_DIR="+claudeDir), "--install-skill")
	if res.code != exitAborted {
		t.Fatalf("exit %d, want %d; stderr: %s", res.code, exitAborted, res.stderr)
	}
	if got := readFile(t, path); got != "hand-written skill\n" {
		t.Error("aborted install overwrote the file")
	}
}
