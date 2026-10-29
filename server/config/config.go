package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Server      ServerConfig   `mapstructure:"server"`
	Logging     LoggingConfig  `mapstructure:"logging"`
	Paths       PathsConfig    `mapstructure:"paths"`
	YtDlp       YtDlpConfig    `mapstructure:"ytdlp"`
	Frontend    FrontendConfig `mapstructure:"frontend"`
	AutoArchive bool           `mapstructure:"auto_archive"`
	Twitch      TwitchConfig   `mapstructure:"twitch"`
	path        string
}

type ServerConfig struct {
	BaseURL   string `mapstructure:"base_url"`
	Host      string `mapstructure:"host"`
	Port      int    `mapstructure:"port"`
	QueueSize int    `mapstructure:"queue_size"`
}

type LoggingConfig struct {
	LogPath           string `mapstructure:"log_path"`
	EnableFileLogging bool   `mapstructure:"enable_file_logging"`

	// DownloadLogPath is where the download access log is written.
	//
	// It is separate from LogPath, and it is not the same kind of thing: the
	// log file carries everything the server does, while this one carries
	// one line per finished download and nothing else. Empty means the
	// default, which is "downloads.log" in the state directory.
	DownloadLogPath string `mapstructure:"download_log_path"`
}

type PathsConfig struct {
	DownloadPath      string `mapstructure:"download_path"`
	DownloaderPath    string `mapstructure:"downloader_path"`
	LocalDatabasePath string `mapstructure:"local_database_path"`
	JSRuntimePath     string `mapstructure:"js_runtime_path"`
	// CookiePath points at a cookie export to convert and pass to yt-dlp.
	//
	// The file may be a browser extension's JSON export or an existing
	// Netscape cookies.txt; the format is detected from the content. It is
	// observed for changes, so replacing the file is enough to refresh the
	// session on a headless host — no restart, no UI step.
	//
	// Relative paths resolve against the config file's directory.
	CookiePath string `mapstructure:"cookie_path"`
}

// YtDlpConfig carries the credentials and network settings every yt-dlp
// invocation needs. They are grouped because they are the knobs an operator
// actually turns when a download fails: the cookie export, the PO Token
// provider, and the egress proxy.
type YtDlpConfig struct {
	// PotBaseURL is the bgutil POT provider endpoint, e.g.
	// http://127.0.0.1:4416. Empty disables the explicit override and yt-dlp
	// falls back to its own config or to no provider.
	PotBaseURL string `mapstructure:"pot_base_url"`
	// Proxy is the proxy the sites that need one are given.
	//
	// It is a seed rather than a fallback: it goes into the profiles of the
	// sites that are reached through a proxy — YouTube, out of here — and
	// not into the others. It used to be the fallback for every site a
	// profile did not name, which sent Bilibili through it as well, and
	// Bilibili answers a request from another country with 412 rather than
	// with the video.
	//
	// A site that needs a different route has one in its own profile.
	Proxy string `mapstructure:"proxy"`
}

type FrontendConfig struct {
	FrontendPath string `mapstructure:"frontend_path"`
}

type TwitchConfig struct {
	ClientId      string        `mapstructure:"client_id"`
	ClientSecret  string        `mapstructure:"client_secret"`
	CheckInterval time.Duration `mapstructure:"check_interval"`
}

var (
	instance     *Config
	instanceOnce sync.Once
)

func Instance() *Config {
	if instance == nil {
		instanceOnce.Do(func() {
			instance = &Config{}
			instance.Twitch.CheckInterval = time.Minute * 5
		})
	}
	return instance
}

// SetInstance replaces the configuration.
//
// It is for the tests, which need a configuration of their own: the proxy and
// the PO Token provider are read from it when a download's arguments are
// built, and a test cannot reach the network to find out.
func SetInstance(c *Config) {
	instanceOnce.Do(func() {})

	instance = c
}

// CookiesPath is the Netscape file yt-dlp reads.
//
// It is derived from the source path, not configured: the converted output
// lives in the state directory under a name keyed by the source, so two
// profiles pointing at different exports cannot overwrite each other, and
// nothing is written next to the export itself.
//
// Both the watcher and the argv builder resolve the target through this
// function. When they derived it independently they disagreed, and the file
// the download read was the one nobody refreshed.
func CookiesPath(source string) string {
	dir := Instance().Paths.LocalDatabasePath
	if dir == "" {
		// No state directory configured (tests, or a partial startup). Fall
		// back to the OS temp dir rather than the working directory: writing
		// converted cookies next to the source tree leaves credential files
		// where nobody expects them and pollutes the repository.
		dir = os.TempDir()
	}

	return filepath.Join(dir, "cookies-"+SourceKey(source)+".txt")
}

// SourceKey derives a short stable key from an export path. A hash rather than
// the basename: two exports can share a name in different directories, and the
// result has to stay valid across restarts.
func SourceKey(source string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(source)))
	return hex.EncodeToString(sum[:])[:12]
}

// ResolvedCookieSource returns paths.cookie_path as an absolute path, or an
// empty string when no source export is configured.
//
// A relative value resolves against the config file's directory, so a tilde-free
// relative path in the YAML means "next to the config" rather than "next to
// whatever directory the process happened to start in".
func ResolvedCookieSource() string {
	p := strings.TrimSpace(Instance().Paths.CookiePath)
	if p == "" {
		return ""
	}

	if strings.HasPrefix(p, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}

	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}

	if dir := Instance().Dir(); dir != "" && dir != "." {
		return filepath.Join(dir, p)
	}

	return filepath.Clean(p)
}

// Path of the directory containing the config file
func (c *Config) Dir() string { return filepath.Dir(c.path) }

// Absolute path of the config file
func (c *Config) Path() string { return c.path }
