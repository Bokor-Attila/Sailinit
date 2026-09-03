package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// updateRepo is the GitHub repository releases are pulled from.
const updateRepo = "Bokor-Attila/Sailinit"

// checksumAsset is the name of the release asset holding sha256 sums.
const checksumAsset = "sha256sums.txt"

// updateAPIBase is overridable in tests to point at a local server.
var updateAPIBase = "https://api.github.com"

var httpClient = &http.Client{Timeout: 30 * time.Second}

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type releaseInfo struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

// asset returns the asset with the given name.
func (r *releaseInfo) asset(name string) (releaseAsset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return releaseAsset{}, false
}

// assetName maps the running platform to its release asset name.
func assetName() (string, error) {
	var osPart string
	switch runtime.GOOS {
	case "darwin":
		osPart = "macos"
	case "linux":
		osPart = "linux"
	default:
		return "", fmt.Errorf("self-update is not supported on %s; download a release manually", runtime.GOOS)
	}

	switch runtime.GOARCH {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("self-update is not supported on %s/%s; download a release manually", runtime.GOOS, runtime.GOARCH)
	}

	return fmt.Sprintf("sailinit-%s-%s", osPart, runtime.GOARCH), nil
}

// parseSemver splits a version like "v1.10.2" into its numeric parts.
func parseSemver(v string) ([3]int, error) {
	var out [3]int
	s := strings.TrimSpace(v)
	s = strings.TrimPrefix(s, "v")
	// Drop any pre-release or build metadata suffix.
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return out, fmt.Errorf("invalid version %q", v)
	}

	parts := strings.Split(s, ".")
	if len(parts) > 3 {
		return out, fmt.Errorf("invalid version %q", v)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, fmt.Errorf("invalid version %q", v)
		}
		out[i] = n
	}
	return out, nil
}

// compareSemver returns -1 if a < b, 0 if equal, 1 if a > b.
// Numeric comparison avoids the lexical trap where "1.10.0" sorts below "1.9.0".
func compareSemver(a, b string) (int, error) {
	av, err := parseSemver(a)
	if err != nil {
		return 0, err
	}
	bv, err := parseSemver(b)
	if err != nil {
		return 0, err
	}
	for i := 0; i < 3; i++ {
		if av[i] < bv[i] {
			return -1, nil
		}
		if av[i] > bv[i] {
			return 1, nil
		}
	}
	return 0, nil
}

// httpGet issues a GET with the User-Agent GitHub requires.
func httpGet(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sailinit/"+version)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s returned %s", url, resp.Status)
	}
	return resp, nil
}

// fetchLatestRelease retrieves metadata for the newest published release.
func fetchLatestRelease() (*releaseInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", strings.TrimSuffix(updateAPIBase, "/"), updateRepo)
	resp, err := httpGet(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rel releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("parsing release metadata: %w", err)
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("release metadata contained no tag name")
	}
	return &rel, nil
}

// fetchChecksums parses the sha256sums.txt asset into name -> hex digest.
// Every release that ships the --upgrade command also ships checksums, so a
// missing file means the release is malformed and the upgrade must not proceed.
func fetchChecksums(rel *releaseInfo) (map[string]string, error) {
	a, ok := rel.asset(checksumAsset)
	if !ok {
		return nil, fmt.Errorf("release %s publishes no %s", rel.TagName, checksumAsset)
	}

	resp, err := httpGet(a.URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	sums := make(map[string]string)
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		// Format: "<digest>  <name>", name may carry a leading "*".
		sums[strings.TrimPrefix(fields[1], "*")] = strings.ToLower(fields[0])
	}
	return sums, nil
}

// downloadAsset streams url into a temp file inside destDir and returns its
// path along with the sha256 of the bytes written. The temp file lives in
// destDir because os.Rename cannot cross filesystems.
func downloadAsset(url, destDir string) (string, string, error) {
	resp, err := httpGet(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	tmp, err := os.CreateTemp(destDir, ".sailinit-update-*")
	if err != nil {
		return "", "", err
	}
	tmpName := tmp.Name()

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hasher), resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return "", "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return "", "", err
	}
	if err := os.Chmod(tmpName, 0755); err != nil {
		os.Remove(tmpName)
		return "", "", err
	}

	return tmpName, hex.EncodeToString(hasher.Sum(nil)), nil
}

// verifyBinary runs the downloaded binary to confirm it is executable on this
// machine and reports the version we expect. Catches truncated downloads and
// architecture mismatches before anything is swapped into place.
func verifyBinary(path, expectedTag string) error {
	out, err := execOutputRunner(path, "", "--version")
	if err != nil {
		return fmt.Errorf("downloaded binary failed to run: %w", err)
	}
	got := strings.TrimSpace(string(out))
	want := strings.TrimPrefix(expectedTag, "v")
	if !strings.Contains(got, want) {
		return fmt.Errorf("downloaded binary reported %q, expected version %s", got, expectedTag)
	}
	return nil
}

// checkWritable reports whether we can create entries in dir.
func checkWritable(dir string) error {
	probe, err := os.CreateTemp(dir, ".sailinit-perm-*")
	if err != nil {
		return err
	}
	name := probe.Name()
	probe.Close()
	return os.Remove(name)
}

// replaceExecutable swaps newPath into exePath, keeping the previous binary
// aside until the swap succeeds so a failure can be rolled back.
func replaceExecutable(newPath, exePath string) error {
	backup := filepath.Join(filepath.Dir(exePath), "."+filepath.Base(exePath)+".old")
	os.Remove(backup)

	if err := os.Rename(exePath, backup); err != nil {
		return fmt.Errorf("moving current binary aside: %w", err)
	}

	if err := os.Rename(newPath, exePath); err != nil {
		// Put the original back before surfacing the failure.
		if restoreErr := os.Rename(backup, exePath); restoreErr != nil {
			return fmt.Errorf("installing new binary failed (%v) and restoring the original failed too (%v); the previous binary is at %s", err, restoreErr, backup)
		}
		return fmt.Errorf("installing new binary: %w", err)
	}

	os.Remove(backup)
	return nil
}

// resolveExecutablePath returns the real path of the running binary, following
// symlinks so package-manager installs replace the binary and not the link.
// It is a var so tests can point it at a fixture.
var resolveExecutablePath = func() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

// runUpgrade downloads and installs the latest release over the running binary.
func runUpgrade(dryRun, assumeYes bool) error {
	if version == "dev" && !assumeYes {
		return fmt.Errorf("this is a development build (version %q); upgrading would replace it with a release build.\nRe-run with --yes if that is what you want", version)
	}

	name, err := assetName()
	if err != nil {
		return err
	}

	exePath, err := resolveExecutablePath()
	if err != nil {
		return fmt.Errorf("locating the running binary: %w", err)
	}

	printInfo("Checking for updates...")
	rel, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	if version != "dev" {
		cmp, err := compareSemver(rel.TagName, version)
		if err != nil {
			return fmt.Errorf("comparing versions: %w", err)
		}
		if cmp <= 0 {
			printSuccess(fmt.Sprintf("Already up to date (%s).", version))
			return nil
		}
	}

	if dryRun {
		printInfo(fmt.Sprintf("[dry-run] Current version: %s", version))
		printInfo(fmt.Sprintf("[dry-run] Latest version:  %s", rel.TagName))
		printInfo(fmt.Sprintf("[dry-run] Would install %s over %s", name, exePath))
		return nil
	}

	asset, ok := rel.asset(name)
	if !ok {
		return fmt.Errorf("release %s has no asset named %s", rel.TagName, name)
	}

	destDir := filepath.Dir(exePath)
	if err := checkWritable(destDir); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("no write access to %s; re-run with sudo", destDir)
		}
		return fmt.Errorf("checking write access to %s: %w", destDir, err)
	}

	sums, err := fetchChecksums(rel)
	if err != nil {
		return fmt.Errorf("fetching checksums: %w", err)
	}

	printInfo(fmt.Sprintf("Downloading %s %s...", name, rel.TagName))
	tmpPath, digest, err := downloadAsset(asset.URL, destDir)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", name, err)
	}
	defer os.Remove(tmpPath)

	want, ok := sums[name]
	if !ok {
		return fmt.Errorf("%s lists no checksum for %s", checksumAsset, name)
	}
	if digest != want {
		return fmt.Errorf("checksum mismatch for %s: got %s, expected %s", name, digest, want)
	}
	printSuccess("Checksum verified.")

	if err := verifyBinary(tmpPath, rel.TagName); err != nil {
		return err
	}

	if err := replaceExecutable(tmpPath, exePath); err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Upgraded sailinit %s -> %s (%s)", version, rel.TagName, exePath))
	return nil
}
