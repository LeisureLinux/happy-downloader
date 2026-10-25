package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Server         ServerConfig   `mapstructure:"server"`
	Logging        LoggingConfig  `mapstructure:"logging"`
	Paths          PathsConfig    `mapstructure:"paths"`
	Authentication AuthConfig     `mapstructure:"authentication"`
	OpenId         OpenIdConfig   `mapstructure:"openid"`
	Frontend       FrontendConfig `mapstructure:"frontend"`
	AutoArchive    bool           `mapstructure:"auto_archive"`
	Twitch         TwitchConfig   `mapstructure:"twitch"`
	path           string
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

type AuthConfig struct {
	RequireAuth  bool   `mapstructure:"require_auth"`
	Username     string `mapstructure:"username"`
	PasswordHash string `mapstructure:"password_hash"`
}

type OpenIdConfig struct {
	UseOpenId      bool     `mapstructure:"use_openid"`
	ProviderURL    string   `mapstructure:"provider_url"`
	ClientId       string   `mapstructure:"client_id"`
	ClientSecret   string   `mapstructure:"client_secret"`
	RedirectURL    string   `mapstructure:"redirect_url"`
	EmailWhitelist []string `mapstructure:"email_whitelist"`
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

// CookiesPath is the Netscape file yt-dlp reads. It is derived, not configured:
// when paths.cookie_path is set the converted output is written here, so the
// source export is never overwritten by yt-dlp's cookie-jar writeback.
//
// It is resolved against LocalDatabasePath rather than the working directory,
// so the file is found regardless of where the process was started from.
func CookiesPath() string {
	return filepath.Join(Instance().Paths.LocalDatabasePath, "cookies.txt")
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
