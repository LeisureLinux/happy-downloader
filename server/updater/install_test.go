package updater

import (
	"os"
	"path/filepath"
	"testing"
)

// The install method has to be detected, because the two are very different
// in practice: a package installation starts in 1.7 seconds and a
// standalone binary in 4.2, and every download spawns yt-dlp.
//
// Replacing a package installation with the standalone binary would make
// every download two and a half seconds slower, to no benefit.
func TestPackageManager(t *testing.T) {
	dir := t.TempDir()

	// A pip installation: a script with a shebang, in a directory that
	// holds pip.
	script := filepath.Join(dir, "yt-dlp")
	if err := os.WriteFile(script, []byte("#!/usr/bin/python3\nimport yt_dlp\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "pip3"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if got := packageManager(script); got == "" {
		t.Error("a script with a shebang was not recognised as a package installation")
	}

	// A standalone binary: an ELF file.
	binary := filepath.Join(dir, "yt-dlp-standalone")
	if err := os.WriteFile(binary, []byte("\x7fELF\x02\x01\x01\x00"), 0o755); err != nil {
		t.Fatal(err)
	}

	if got := packageManager(binary); got != "" {
		t.Errorf("a binary was recognised as a package installation: %q", got)
	}

	// A file that is not there.
	if got := packageManager(filepath.Join(dir, "missing")); got != "" {
		t.Errorf("a missing file was recognised as a package installation: %q", got)
	}
}

// uv is preferred over pip when both are there, because it is the one that
// was used to create the environment.
func TestPackageManagerPrefersUv(t *testing.T) {
	dir := t.TempDir()

	script := filepath.Join(dir, "yt-dlp")
	if err := os.WriteFile(script, []byte("#!/usr/bin/python3\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"pip3", "uv"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got := packageManager(script)
	if filepath.Base(got) != "uv" {
		t.Errorf("packageManager picked %q, want uv", filepath.Base(got))
	}
}

// The environment uv installs into is the one yt-dlp belongs to, not the one
// the uv binary happens to live next to.
//
// A venv created by uv has no uv of its own — the one that made it is
// elsewhere on the path — so looking next to the uv binary would find the
// wrong environment and install into it.
func TestPythonOfFindsTheEnvironment(t *testing.T) {
	dir := t.TempDir()

	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}

	// .../venv/bin/yt-dlp
	ytdlp := filepath.Join(bin, "yt-dlp")
	if err := os.WriteFile(ytdlp, []byte("#!/usr/bin/python3\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// .../venv/bin/python3
	python := filepath.Join(bin, "python3")
	if err := os.WriteFile(python, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := pythonOf(ytdlp)
	if got != python {
		t.Errorf("pythonOf = %q, want %q", got, python)
	}
}
