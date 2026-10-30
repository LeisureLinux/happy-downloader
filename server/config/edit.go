package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"
)

// The settings the interface can change.
//
// The configuration is read from a file when the server starts, so a setting
// that is changed in the interface would be lost at the next restart unless
// it is written back. These two are worth changing from the interface — the
// PO Token provider is only useful for YouTube and the proxy is a property of
// the installation — so they are written back to the file.
//
// The file is rewritten as a whole, with the keys that were not there added
// and the ones that were kept. A comment in the file is lost, which is the
// price of a file that can be edited by a program; the file the server wrote
// is the one it reads.

// The path of the configuration file, as it was given on the command line.
var (
	pathMu   sync.RWMutex
	filePath string
)

// SetFilePath records where the configuration was read from.
//
// It is set once at startup, and the settings that the interface can change
// are written back to it.
func SetFilePath(path string) {
	pathMu.Lock()
	filePath = path
	pathMu.Unlock()
}

// FilePath returns where the configuration is read from.
func FilePath() string {
	pathMu.RLock()
	defer pathMu.RUnlock()

	return filePath
}

// The settings the interface can change.
type Editable struct {
	// PotBaseURL is the bgutil PO Token provider.
	PotBaseURL string `yaml:"pot_base_url"`
	// Proxy is the proxy for the sites that need one.
	Proxy string `yaml:"proxy"`
	// DownloadPath is where the finished files go.
	//
	// It is here because the default was wrong on a desktop install: the
	// application was started from the folder it was unpacked into, so every
	// download landed beside the executable — in "Program Files" on a
	// machine where the person has no business writing, and in the place the
	// uninstaller will one day delete.
	//
	// An empty value means the default is kept.
	DownloadPath string `yaml:"download_path"`
}

// Update writes the settings that the interface can change.
//
// The file is read, the two settings are set, and it is written back. The
// rest of the file — including anything the operator wrote by hand — is kept.
func Update(editable Editable) error {
	path := FilePath()
	if path == "" {
		return errNoConfigFile
	}

	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// The file is read into a map so that the parts of it that this does
	// not know about are kept as they are.
	var doc map[string]any
	if len(content) > 0 {
		if err := yaml.Unmarshal(content, &doc); err != nil {
			return err
		}
	}

	if doc == nil {
		doc = map[string]any{}
	}

	ytdlp, _ := doc["ytdlp"].(map[string]any)
	if ytdlp == nil {
		ytdlp = map[string]any{}
	}

	ytdlp["pot_base_url"] = editable.PotBaseURL
	ytdlp["proxy"] = editable.Proxy

	doc["ytdlp"] = ytdlp

	// The download directory is under "paths", and it is only written when
	// it was given: an empty value means "keep what is there", so a form
	// that does not know about the setting cannot empty it by saving.
	if strings.TrimSpace(editable.DownloadPath) != "" {
		paths, _ := doc["paths"].(map[string]any)
		if paths == nil {
			paths = map[string]any{}
		}

		paths["download_path"] = strings.TrimSpace(editable.DownloadPath)
		doc["paths"] = paths
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}

	// A temporary name and a rename, so a reader never sees half a file.
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}

	// The values that are read at runtime are updated too, so the change
	// takes effect without a restart.
	instance := Instance()
	instance.YtDlp.PotBaseURL = strings.TrimSpace(editable.PotBaseURL)
	instance.YtDlp.Proxy = strings.TrimSpace(editable.Proxy)

	// The download directory takes effect at once: a change that needs a
	// restart is a change nobody makes.
	if dir := strings.TrimSpace(editable.DownloadPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}

		instance.Paths.DownloadPath = dir
	}

	return nil
}

// errNoConfigFile is returned when there is no file to write to.
var errNoConfigFile = errConfig("the configuration file is not known")

type errConfig string

func (e errConfig) Error() string { return string(e) }

// ConfigDir returns the directory the configuration file is in.
func ConfigDir() string {
	path := FilePath()
	if path == "" {
		return ""
	}

	return filepath.Dir(path)
}
