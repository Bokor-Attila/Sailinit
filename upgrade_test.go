package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeRelease spins up a server serving release metadata, the platform asset
// and (optionally) a checksum file. It returns the server and the asset bytes.
type fakeRelease struct {
	tag          string
	assetBody    []byte
	withChecksum bool
	badChecksum  bool
	omitAsset    bool
	// checksumOmitsAsset serves a sha256sums.txt that lists some other file
	// but not this platform's binary.
	checksumOmitsAsset bool
	// downloads counts how many times the platform binary was fetched.
	downloads *int32
}

func (f fakeRelease) start(t *testing.T) *httptest.Server {
	t.Helper()

	name, err := assetName()
	if err != nil {
		t.Fatalf("assetName: %v", err)
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)

	sum := sha256.Sum256(f.assetBody)
	digest := hex.EncodeToString(sum[:])
	if f.badChecksum {
		digest = strings.Repeat("0", 64)
	}

	mux.HandleFunc("/repos/"+updateRepo+"/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var assets []string
		if !f.omitAsset {
			assets = append(assets, fmt.Sprintf(`{"name":%q,"browser_download_url":%q}`, name, srv.URL+"/dl/"+name))
		}
		if f.withChecksum {
			assets = append(assets, fmt.Sprintf(`{"name":%q,"browser_download_url":%q}`, checksumAsset, srv.URL+"/dl/"+checksumAsset))
		}
		fmt.Fprintf(w, `{"tag_name":%q,"assets":[%s]}`, f.tag, strings.Join(assets, ","))
	})

	mux.HandleFunc("/dl/"+name, func(w http.ResponseWriter, r *http.Request) {
		if f.downloads != nil {
			atomic.AddInt32(f.downloads, 1)
		}
		w.Write(f.assetBody)
	})

	mux.HandleFunc("/dl/"+checksumAsset, func(w http.ResponseWriter, r *http.Request) {
		if f.checksumOmitsAsset {
			fmt.Fprintf(w, "%s  some-other-file\n", digest)
			return
		}
		fmt.Fprintf(w, "%s  %s\n", digest, name)
	})

	t.Cleanup(srv.Close)
	return srv
}

// installFixture creates a fake installed binary in a temp dir and points
// resolveExecutable's inputs at it by returning the path.
func installFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "sailinit")
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

// withUpgradeEnv wires the package-level seams for one test and restores them.
func withUpgradeEnv(t *testing.T, apiBase, ver, exePath, reportVersion string) {
	t.Helper()

	// A successful upgrade consults the Claude skill tracking file; keep it
	// away from the real one.
	t.Setenv("SAILINIT_HOME", t.TempDir())

	origAPI := updateAPIBase
	origVersion := version
	origOutputRunner := execOutputRunner
	origResolve := resolveExecutablePath

	updateAPIBase = apiBase
	version = ver
	resolveExecutablePath = func() (string, error) { return exePath, nil }
	execOutputRunner = func(name string, dir string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "--version" {
			if reportVersion == "" {
				return nil, fmt.Errorf("exec format error")
			}
			return []byte("sailinit " + reportVersion), nil
		}
		return []byte(""), nil
	}

	t.Cleanup(func() {
		updateAPIBase = origAPI
		version = origVersion
		execOutputRunner = origOutputRunner
		resolveExecutablePath = origResolve
	})
}

func TestParseSemverAndCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"v1.10.0", "v1.9.0", 1},
		{"1.9.0", "1.10.0", -1},
		{"v1.4.0", "1.4.0", 0},
		{"v2.0.0", "v1.99.99", 1},
		{"v1.4.1", "v1.4.0", 1},
		{"v1.5", "v1.5.0", 0},
		{"v1.5.0-beta", "v1.5.0", 0},
	}

	for _, tc := range tests {
		got, err := compareSemver(tc.a, tc.b)
		if err != nil {
			t.Fatalf("compareSemver(%q, %q) errored: %v", tc.a, tc.b, err)
		}
		if got != tc.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}

	for _, bad := range []string{"", "dev", "v", "1.x.0", "a.b.c", "1.2.3.4"} {
		if _, err := compareSemver(bad, "1.0.0"); err == nil {
			t.Errorf("compareSemver(%q, ...) expected an error, got nil", bad)
		}
	}
}

func TestAssetName(t *testing.T) {
	name, err := assetName()
	if err != nil {
		t.Fatalf("assetName on this platform errored: %v", err)
	}
	if !strings.HasPrefix(name, "sailinit-") {
		t.Errorf("asset name %q missing sailinit- prefix", name)
	}
	// darwin must map to "macos", never the raw GOOS.
	if strings.Contains(name, "darwin") {
		t.Errorf("asset name %q should use macos, not darwin", name)
	}
}

func TestFetchLatestRelease(t *testing.T) {
	srv := fakeRelease{tag: "v9.9.9", assetBody: []byte("binary")}.start(t)
	updateAPIBaseSwap(t, srv.URL)

	rel, err := fetchLatestRelease()
	if err != nil {
		t.Fatalf("fetchLatestRelease: %v", err)
	}
	if rel.TagName != "v9.9.9" {
		t.Errorf("tag = %q, want v9.9.9", rel.TagName)
	}
	name, _ := assetName()
	if _, ok := rel.asset(name); !ok {
		t.Errorf("release is missing asset %q", name)
	}
}

func TestFetchLatestReleaseErrors(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer bad.Close()
	updateAPIBaseSwap(t, bad.URL)
	if _, err := fetchLatestRelease(); err == nil {
		t.Error("expected an error on HTTP 404, got nil")
	}

	garbage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "{not json")
	}))
	defer garbage.Close()
	updateAPIBaseSwap(t, garbage.URL)
	if _, err := fetchLatestRelease(); err == nil {
		t.Error("expected an error on malformed JSON, got nil")
	}
}

func updateAPIBaseSwap(t *testing.T, url string) {
	t.Helper()
	orig := updateAPIBase
	updateAPIBase = url
	t.Cleanup(func() { updateAPIBase = orig })
}

func TestUpgradeAlreadyUpToDate(t *testing.T) {
	srv := fakeRelease{tag: "v1.4.0", assetBody: []byte("new binary")}.start(t)
	exe := installFixture(t, "old binary")
	withUpgradeEnv(t, srv.URL, "v1.4.0", exe, "1.4.0")

	if err := runUpgrade(false, false); err != nil {
		t.Fatalf("runUpgrade: %v", err)
	}
	assertFileContains(t, exe, "old binary")
	assertNoLeftovers(t, filepath.Dir(exe))
}

func TestUpgradeNewerLocalVersionDoesNotDowngrade(t *testing.T) {
	srv := fakeRelease{tag: "v1.4.0", assetBody: []byte("new binary")}.start(t)
	exe := installFixture(t, "old binary")
	withUpgradeEnv(t, srv.URL, "v1.10.0", exe, "1.4.0")

	if err := runUpgrade(false, false); err != nil {
		t.Fatalf("runUpgrade: %v", err)
	}
	assertFileContains(t, exe, "old binary")
}

func TestUpgradeRefusesDevBuild(t *testing.T) {
	srv := fakeRelease{tag: "v9.9.9", assetBody: []byte("new binary")}.start(t)
	exe := installFixture(t, "old binary")
	withUpgradeEnv(t, srv.URL, "dev", exe, "9.9.9")

	err := runUpgrade(false, false)
	if err == nil {
		t.Fatal("expected a dev build to be refused, got nil")
	}
	if !strings.Contains(err.Error(), "development build") {
		t.Errorf("error %q should mention a development build", err)
	}
	assertFileContains(t, exe, "old binary")
}

func TestUpgradeDryRunChangesNothing(t *testing.T) {
	srv := fakeRelease{tag: "v9.9.9", assetBody: []byte("new binary"), withChecksum: true}.start(t)
	exe := installFixture(t, "old binary")
	withUpgradeEnv(t, srv.URL, "v1.4.0", exe, "9.9.9")

	if err := runUpgrade(true, false); err != nil {
		t.Fatalf("runUpgrade dry-run: %v", err)
	}
	assertFileContains(t, exe, "old binary")
	assertNoLeftovers(t, filepath.Dir(exe))
}

func TestUpgradeChecksumMismatchAborts(t *testing.T) {
	srv := fakeRelease{tag: "v9.9.9", assetBody: []byte("new binary"), withChecksum: true, badChecksum: true}.start(t)
	exe := installFixture(t, "old binary")
	withUpgradeEnv(t, srv.URL, "v1.4.0", exe, "9.9.9")

	err := runUpgrade(false, false)
	if err == nil {
		t.Fatal("expected a checksum mismatch to abort the upgrade, got nil")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("error %q should mention a checksum mismatch", err)
	}
	assertFileContains(t, exe, "old binary")
	assertNoLeftovers(t, filepath.Dir(exe))
}

func TestUpgradeVerifyFailureAborts(t *testing.T) {
	srv := fakeRelease{tag: "v9.9.9", assetBody: []byte("truncated"), withChecksum: true}.start(t)
	exe := installFixture(t, "old binary")
	// reportVersion empty => the downloaded binary fails to exec.
	withUpgradeEnv(t, srv.URL, "v1.4.0", exe, "")

	err := runUpgrade(false, false)
	if err == nil {
		t.Fatal("expected a failed binary verification to abort the upgrade, got nil")
	}
	if !strings.Contains(err.Error(), "failed to run") {
		t.Errorf("error %q should mention the binary failing to run", err)
	}
	assertFileContains(t, exe, "old binary")
	assertNoLeftovers(t, filepath.Dir(exe))
}

func TestUpgradeVerifyWrongVersionAborts(t *testing.T) {
	srv := fakeRelease{tag: "v9.9.9", assetBody: []byte("wrong build"), withChecksum: true}.start(t)
	exe := installFixture(t, "old binary")
	withUpgradeEnv(t, srv.URL, "v1.4.0", exe, "1.2.3")

	err := runUpgrade(false, false)
	if err == nil {
		t.Fatal("expected a version mismatch to abort the upgrade, got nil")
	}
	assertFileContains(t, exe, "old binary")
}

func TestUpgradeMissingAssetAborts(t *testing.T) {
	srv := fakeRelease{tag: "v9.9.9", assetBody: []byte("new binary"), omitAsset: true}.start(t)
	exe := installFixture(t, "old binary")
	withUpgradeEnv(t, srv.URL, "v1.4.0", exe, "9.9.9")

	err := runUpgrade(false, false)
	if err == nil {
		t.Fatal("expected a missing platform asset to abort the upgrade, got nil")
	}
	if !strings.Contains(err.Error(), "no asset named") {
		t.Errorf("error %q should name the missing asset", err)
	}
	assertFileContains(t, exe, "old binary")
}

func TestUpgradeHappyPath(t *testing.T) {
	srv := fakeRelease{tag: "v9.9.9", assetBody: []byte("new binary"), withChecksum: true}.start(t)
	exe := installFixture(t, "old binary")
	withUpgradeEnv(t, srv.URL, "v1.4.0", exe, "9.9.9")

	if err := runUpgrade(false, false); err != nil {
		t.Fatalf("runUpgrade: %v", err)
	}

	assertFileContains(t, exe, "new binary")

	info, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("installed binary mode = %v, want 0755", info.Mode().Perm())
	}
	assertNoLeftovers(t, filepath.Dir(exe))
}

func TestUpgradeWithoutChecksumsAborts(t *testing.T) {
	var downloads int32
	srv := fakeRelease{tag: "v9.9.9", assetBody: []byte("new binary"), downloads: &downloads}.start(t)
	exe := installFixture(t, "old binary")
	withUpgradeEnv(t, srv.URL, "v1.4.0", exe, "9.9.9")

	err := runUpgrade(false, false)
	if err == nil {
		t.Fatal("expected a release without checksums to abort the upgrade, got nil")
	}
	if !strings.Contains(err.Error(), "publishes no "+checksumAsset) {
		t.Errorf("error %q should report that the release publishes no %s", err, checksumAsset)
	}
	// The abort must happen before any bytes are pulled down.
	if n := atomic.LoadInt32(&downloads); n != 0 {
		t.Errorf("binary was downloaded %d time(s); a missing %s must abort first", n, checksumAsset)
	}
	assertFileContains(t, exe, "old binary")
	assertNoLeftovers(t, filepath.Dir(exe))
}

// A release that ships sha256sums.txt but omits our platform's line must also
// abort rather than install an unverified binary.
func TestUpgradeChecksumFileMissingOurAssetAborts(t *testing.T) {
	name, err := assetName()
	if err != nil {
		t.Fatal(err)
	}

	srv := fakeRelease{tag: "v9.9.9", assetBody: []byte("new binary"), withChecksum: true, checksumOmitsAsset: true}.start(t)
	exe := installFixture(t, "old binary")
	withUpgradeEnv(t, srv.URL, "v1.4.0", exe, "9.9.9")

	err = runUpgrade(false, false)
	if err == nil {
		t.Fatal("expected a missing checksum line to abort the upgrade, got nil")
	}
	if !strings.Contains(err.Error(), name) {
		t.Errorf("error %q should name the asset %s", err, name)
	}
	assertFileContains(t, exe, "old binary")
	assertNoLeftovers(t, filepath.Dir(exe))
}

func TestUpgradeReadOnlyDirSuggestsSudo(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; directory permissions do not apply")
	}

	srv := fakeRelease{tag: "v9.9.9", assetBody: []byte("new binary"), withChecksum: true}.start(t)
	exe := installFixture(t, "old binary")
	dir := filepath.Dir(exe)
	withUpgradeEnv(t, srv.URL, "v1.4.0", exe, "9.9.9")

	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0755) })

	err := runUpgrade(false, false)
	if err == nil {
		t.Fatal("expected a read-only install directory to abort the upgrade, got nil")
	}
	if !strings.Contains(err.Error(), "sudo") {
		t.Errorf("error %q should suggest re-running with sudo", err)
	}
	assertFileContains(t, exe, "old binary")
}

func TestReplaceExecutableRestoresOnFailure(t *testing.T) {
	exe := installFixture(t, "old binary")

	// Renaming a source that does not exist fails, so the original binary
	// must be restored from the backup.
	src := filepath.Join(filepath.Dir(exe), "does-not-exist")

	if err := replaceExecutable(src, exe); err == nil {
		t.Fatal("expected replaceExecutable to fail, got nil")
	}
	assertFileContains(t, exe, "old binary")
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("%s contains %q, want %q", path, got, want)
	}
}

// assertNoLeftovers fails if any temp or backup file survived the run.
func assertNoLeftovers(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, ".sailinit-update-") || strings.HasSuffix(n, ".old") || strings.HasPrefix(n, ".sailinit-perm-") {
			t.Errorf("leftover file in install dir: %s", n)
		}
	}
}
