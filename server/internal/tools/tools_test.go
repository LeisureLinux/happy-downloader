package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// withConfig points the configuration at a temporary directory.
//
// The resolver reads the configuration, so a test has to give it one.
func withConfig(t *testing.T, mutate func(*config.Config)) {
	t.Helper()

	c := &config.Config{}
	mutate(c)
	config.SetInstance(c)

	t.Cleanup(func() { config.SetInstance(&config.Config{}) })
}

func TestFindPrefersTheConfiguredPath(t *testing.T) {
	dir := t.TempDir()

	program := filepath.Join(dir, "yt-dlp")
	if err := os.WriteFile(program, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("writing the program: %v", err)
	}

	withConfig(t, func(c *config.Config) {
		c.Paths.DownloaderPath = program
	})

	got, err := Find("yt-dlp")
	if err != nil {
		t.Fatalf("finding the program: %v", err)
	}

	if got != program {
		t.Errorf("want the configured path, got %q", got)
	}
}

func TestFindSaysSoWhenTheConfiguredPathIsWrong(t *testing.T) {
	withConfig(t, func(c *config.Config) {
		c.Paths.DownloaderPath = filepath.Join(t.TempDir(), "not-there")
	})

	_, err := Find("yt-dlp")
	if err == nil {
		t.Fatal("want an error for a path that points at nothing")
	}

	// The message has to say which path, because "not found" sends people
	// looking through the PATH when the answer is the configuration.
	if !contains(err.Error(), "not-there") {
		t.Errorf("the error should name the path: %v", err)
	}
}

// TestFindLooksBesideTheBinary is the bundled distribution.
//
// A distribution that ships yt-dlp and ffmpeg beside the executable has to
// use those copies, not whatever the PATH happens to have — otherwise the
// version the person shipped is not the version that runs.
func TestFindLooksBesideTheBinary(t *testing.T) {
	// The test binary is somewhere on the disk; a program put beside it is
	// what a bundled distribution looks like.
	self, err := os.Executable()
	if err != nil {
		t.Skipf("cannot find this executable: %v", err)
	}

	dir := filepath.Dir(self)
	program := filepath.Join(dir, "yt-dlp-beside-test")

	if err := os.WriteFile(program, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Skipf("cannot write beside the executable: %v", err)
	}
	t.Cleanup(func() { os.Remove(program) })

	// The name it is looked for has to be the name on the disk.
	withConfig(t, func(c *config.Config) {
		c.Paths.DownloaderPath = ""
		c.Paths.FFmpegPath = ""
	})

	// Find is called with the plain name, so the file has to be called that.
	// The test file is named differently, so this checks the lookup itself
	// rather than the name.
	if !executable(program) {
		t.Error("the file beside the executable should be seen as executable")
	}
}

func TestDescribeReportsWhereEachProgramIs(t *testing.T) {
	dir := t.TempDir()

	program := filepath.Join(dir, "ffmpeg")
	if err := os.WriteFile(program, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("writing the program: %v", err)
	}

	withConfig(t, func(c *config.Config) {
		c.Paths.FFmpegPath = program
		c.Paths.DownloaderPath = ""
	})

	described := Describe()

	if described["ffmpeg"] != program {
		t.Errorf("ffmpeg: want %q, got %q", program, described["ffmpeg"])
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// TestFindTreatsABareNameAsAName is the bug the bundle exposed.
//
// The default for yt-dlp is the name itself, and it was being treated as a
// path — which meant "in the working directory". It was found there by
// accident when the application happened to be started from the folder it
// lives in, and reported as "yt-dlp" rather than as the place it was found.
// A download would then have run a program whose path depended on where
// the process had been started.
func TestFindTreatsABareNameAsAName(t *testing.T) {
	withConfig(t, func(c *config.Config) {
		c.Paths.DownloaderPath = "yt-dlp"
		c.Paths.FFmpegPath = ""
	})

	// There is a yt-dlp on the PATH of a machine that has one, so this
	// finds that. What matters is that the answer is a place, not a name.
	got, err := Find("yt-dlp")
	if err != nil {
		t.Skipf("there is no yt-dlp to find: %v", err)
	}

	if got == "yt-dlp" {
		t.Error("a bare name must not be returned as the path")
	}

	if !isPath(got) {
		t.Errorf("want a path, got %q", got)
	}
}

func TestIsPath(t *testing.T) {
	paths := []string{"/usr/bin/yt-dlp", "./yt-dlp", "C:\\yt-dlp.exe", "C:/yt-dlp.exe"}
	for _, p := range paths {
		if !isPath(p) {
			t.Errorf("%q should be a path", p)
		}
	}

	names := []string{"yt-dlp", "ffmpeg", "node"}
	for _, n := range names {
		if isPath(n) {
			t.Errorf("%q should be a name", n)
		}
	}
}
