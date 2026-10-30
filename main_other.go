//go:build !windows

package main

// The desktop behaviour that only Windows needs, as no-ops elsewhere.
//
// On Linux and macOS the program is started from a terminal or from a service
// manager, and both of them are already the answer to "where does the output
// go" and "what is the address". Nothing has to be opened or shown, so these
// do nothing.

func setupDesktop() {}

// whenReady reports the address, and leaves opening it to the person.
//
// Opening a browser from a server would be wrong here: the program is
// usually started on a machine nobody is looking at, and the browser it would
// open is not necessarily the one the person is using.
func whenReady(url string) {}

// whenFatal is silent, because the log already said it.
//
// A message box on a headless machine would wait for a click that never
// comes.
func whenFatal(message string) {}
