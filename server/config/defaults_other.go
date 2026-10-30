//go:build !windows

package config

// The data directory on a unix-like system is the working directory.
//
// That is not a good default in general, but it is the one the deployed
// installations already have: the service sets a working directory and the
// database is beside it. Changing where the files of a running deployment
// live would lose its history, so this stays as it is and only the Windows
// default is different — on Windows, "the working directory" is a folder the
// user usually cannot write to.
func DefaultDataDir() string { return "" }

// The configuration file, in the working directory.
func DefaultConfigPath() string { return "./config.yml" }
