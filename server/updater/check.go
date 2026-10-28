package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// Checking whether yt-dlp has a newer version, and installing it.
//
// yt-dlp breaks YouTube every few weeks, and the fix is a new binary rather
// than a new build of this application. So the version is worth checking from
// the interface: the footer shows it, and a marker next to it says there is
// something newer.
//
// Two sources of truth, and they answer different questions:
//
//   - yt-dlp itself knows what "latest" means for the channel it was
//     installed from — stable, nightly, or a specific release. Its
//     --update-to with --simulate reports the current and the latest version
//     and changes nothing.
//   - GitHub's releases API knows what the newest release is.
//
// The first is authoritative about "newer than what I have", because it knows
// the channel. The second is the fallback when the first cannot run — and it
// is what the update itself is installed from, since the GitHub binary is the
// one that can be replaced without a package manager.

// The GitHub API endpoint for the newest release.
const latestReleaseURL = "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest"

// Status says whether a newer yt-dlp exists.
type Status struct {
	// Current is the version that is installed.
	Current string `json:"current"`
	// Latest is the version that is available, when it could be determined.
	Latest string `json:"latest,omitempty"`
	// Outdated says whether Latest is newer than Current.
	Outdated bool `json:"outdated"`
	// Error is why the check failed, when it did.
	Error string `json:"error,omitempty"`
}

// Check reports whether a newer yt-dlp is available.
//
// It is a network call, so it is not something to do on every page load: the
// caller decides how often, and the answer is cached by the caller.
func Check(ctx context.Context) (*Status, error) {
	status := &Status{Current: CurrentVersion()}

	latest, err := latestVersion(ctx)
	if err != nil {
		status.Error = err.Error()
		return status, err
	}

	status.Latest = latest
	status.Outdated = isNewer(latest, status.Current)

	return status, nil
}

// CurrentVersion returns the version of the installed yt-dlp.
func CurrentVersion() string {
	out, err := exec.Command(config.Instance().Paths.DownloaderPath, "--version").Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(strings.TrimPrefix(string(out), "v"))
}

// latestVersion asks GitHub for the newest release.
//
// The API is used rather than yt-dlp's own --update-to because the update
// itself is installed from the GitHub binary, so the release that matters is
// the release on GitHub. It is also a single small request, where
// --update-to spawns a process that resolves the channel and then refuses to
// install anything.
func latestVersion(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return "", err
	}

	// The API rejects a request without a user agent.
	req.Header.Set("User-Agent", "happy-downloader")
	req.Header.Set("Accept", "application/vnd.github+json")

	// Through the proxy the host is configured with. Without it the check
	// times out on a host that cannot reach GitHub directly, and the
	// answer is that yt-dlp is fine — which is not the same as knowing
	// whether there is an update.
	res, err := httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the release check returned %s", res.Status)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}

	var release struct {
		TagName string `json:"tag_name"`
	}

	if err := json.Unmarshal(body, &release); err != nil {
		return "", err
	}

	version := strings.TrimSpace(strings.TrimPrefix(release.TagName, "v"))
	if version == "" {
		return "", fmt.Errorf("the release check returned no version")
	}

	return version, nil
}

// assetName returns the name of the release asset for an architecture.
func assetName(arch string) (string, bool) {
	switch arch {
	case "amd64":
		return "yt-dlp_linux", true
	case "arm64":
		return "yt-dlp_linux_aarch64", true
	case "arm":
		return "yt-dlp_linux_armv7l", true
	default:
		return "", false
	}
}

// isNewer compares two version strings.
//
// The versions are dates — "2026.10.21" — so they compare as strings, once
// the parts are the same width. A release candidate suffix is ignored: the
// daily builds are named "2026.10.21.123456" and the release is
// "2026.10.21", and the release is what is wanted.
func isNewer(latest, current string) bool {
	if latest == "" || current == "" {
		return false
	}

	latest = trimBuild(latest)
	current = trimBuild(current)

	if latest == current {
		return false
	}

	// The date form is fixed width, so a string comparison is the same as a
	// numeric one. Anything that is not that form is not compared: guessing
	// would say "newer" about a version that is not.
	if !isDateForm(latest) || !isDateForm(current) {
		return false
	}

	return latest > current
}

// trimBuild removes the build suffix a daily build carries.
func trimBuild(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) > 3 {
		return strings.Join(parts[:3], ".")
	}

	return version
}

// isDateForm reports whether a version is the "YYYY.MM.DD" form.
func isDateForm(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}

	for _, p := range parts {
		if len(p) != 2 && len(p) != 4 {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}

	return true
}

// Update installs a newer yt-dlp.
//
// How it is installed depends on how it is already installed, because the
// two are very different in practice:
//
//   - A pip or uv installation is updated by the package manager. That is
//     what it was installed with, and the package manager is the only thing
//     that can replace it correctly.
//   - A standalone binary is replaced by downloading the new one.
//
// The standalone binary is a self-contained bundle that unpacks itself on
// every run, and it takes 4.2 seconds to start against the package
// installation's 1.7. Every download spawns yt-dlp, so the difference is
// paid on every download — which is why the install method is kept rather
// than always using the binary.
//
// The replacement of a binary is atomic: the new one is written next to the
// old one and renamed over it, so a download starting at that moment finds
// either the old binary or the new one, never a half-written file.
func Update(ctx context.Context) error {
	current := config.Instance().Paths.DownloaderPath

	if current == "" {
		return fmt.Errorf("no yt-dlp path is configured")
	}

	version, err := latestVersion(ctx)
	if err != nil {
		return err
	}

	if manager := packageManager(current); manager != "" {
		return updateWithPackageManager(ctx, manager, current, version)
	}

	// The asset name depends on the architecture.
	//
	// The release carries one binary per platform, and "yt-dlp_linux" is the
	// x86-64 one — so a host that is not x86-64 would have had its binary
	// replaced with one it cannot run. The names are:
	//
	//   x86-64   yt-dlp_linux
	//   arm64    yt-dlp_linux_aarch64
	//   armv7    yt-dlp_linux_armv7l
	//
	// Anything else has no Linux binary, and the update says so rather than
	// installing something that will not run.
	asset, ok := assetName(runtime.GOARCH)
	if !ok {
		return fmt.Errorf("yt-dlp has no Linux binary for %s", runtime.GOARCH)
	}

	url := fmt.Sprintf(
		"https://github.com/yt-dlp/yt-dlp/releases/download/%s/%s", version, asset)

	slog.Info("downloading a new yt-dlp",
		slog.String("version", version),
		slog.String("asset", asset),
	)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	// Through the proxy, for the same reason as the check: the binary comes
	// from GitHub, and a host that reaches the internet through a proxy
	// cannot reach GitHub directly.
	res, err := httpClient().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading yt-dlp returned %s", res.Status)
	}

	tmp := current + ".new"

	file, err := createExecutable(tmp)
	if err != nil {
		return err
	}

	if _, err := io.Copy(file, res.Body); err != nil {
		file.Close()
		removeQuietly(tmp)
		return err
	}

	if err := file.Close(); err != nil {
		removeQuietly(tmp)
		return err
	}

	// The new binary is run before it is put in place.
	//
	// A wrong asset or a truncated download would otherwise replace a
	// working yt-dlp with one that cannot start, and the next download
	// would fail with a message about the downloader rather than about the
	// update. Asking the new binary for its version proves it runs on this
	// machine, which is also the proof that the architecture is right.
	if err := verifyBinary(tmp); err != nil {
		removeQuietly(tmp)
		return err
	}

	if err := replaceFile(tmp, current); err != nil {
		removeQuietly(tmp)
		return err
	}

	slog.Info("yt-dlp updated", slog.String("version", version))

	return nil
}
