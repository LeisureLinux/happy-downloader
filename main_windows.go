//go:build windows

package main

import (
	"log/slog"
	"os"
	"syscall"
	"unsafe"
)

// The Windows build has no console.
//
// It is linked with "-H windowsgui", which means there is no black window: the
// program runs, the port is open, and the files download. That is what a
// desktop application should look like — but it also means there is nowhere to
// print anything, and a program that fails silently is worse than one that
// fails loudly.
//
// So the two things a console would have done are done here instead: the
// browser is opened at the address, and a failure before the port is open is
// said in a message box.

var (
	user32       = syscall.NewLazyDLL("user32.dll")
	shell32      = syscall.NewLazyDLL("shell32.dll")
	kernel32     = syscall.NewLazyDLL("kernel32.dll")
	procMessageB = user32.NewProc("MessageBoxW")
	procShellRun = shell32.NewProc("ShellExecuteW")
	procAttach   = kernel32.NewProc("AttachConsole")
)

const (
	mbIconError    = 0x00000010
	mbSetForegound = 0x00010000
	attachParent   = ^uintptr(0) // (DWORD)-1, the parent process
	swShownNormal  = 1
)

// openURL asks the shell to open the address, which is what double-clicking a
// link does: the default browser, however it is set up, with no assumption
// about where it is installed.
func openURL(url string) {
	verb, _ := syscall.UTF16PtrFromString("open")
	target, _ := syscall.UTF16PtrFromString(url)

	// The return value is an HINSTANCE that encodes an error above 32. It is
	// not checked: there is nothing useful to do about "no browser" from
	// here, and the address is in the log for anyone who needs it.
	_, _, _ = procShellRun.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(target)),
		0, 0, swShownNormal,
	)
}

// tellUser says something to the person who started the program.
//
// A message box rather than a print, because the print would go nowhere: this
// build has no console to print to.
func tellUser(title, message string) {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(message)

	_, _, _ = procMessageB.Call(
		0,
		uintptr(unsafe.Pointer(m)),
		uintptr(unsafe.Pointer(t)),
		mbIconError|mbSetForegound,
	)
}

// startedFromTerminal says whether there was somewhere to print.
//
// A program that was started from a terminal is being watched by someone who
// can read its output, so it does not open a browser at them: they asked for
// a program in a terminal, and that is what they should get.
var startedFromTerminal bool

// attachConsole lets the same binary be useful from a command prompt.
//
// A windowsgui program started from cmd does not inherit the console, so
// running it by hand would print nothing and return at once. Attaching to the
// parent console gives back the behaviour people expect from a terminal
// program, without giving the window to someone who double-clicked it.
func attachConsole() {
	// A redirected standard output is already somewhere to write — a log
	// file, a pipe, a test harness. It is left alone: replacing it with the
	// console would send the output to a console the caller cannot see.
	if _, err := os.Stdout.Stat(); err == nil {
		startedFromTerminal = true
		return
	}

	if _, _, err := procAttach.Call(attachParent); err != nil {
		// No parent console — started from Explorer, or from a shortcut.
		// That is the normal case for the desktop build, and its standard
		// output is an invalid handle rather than a stream.
		//
		// It is pointed at the null device so that writes to it succeed
		// instead of failing on every line. The log has other destinations,
		// and every one of them used to depend on this one being writable.
		if devnull, err := os.OpenFile("NUL", os.O_WRONLY, 0); err == nil {
			os.Stdout = devnull
			os.Stderr = devnull
		}

		return
	}

	// The standard handles are inherited, not created, when a console is
	// attached after the process started.
	if out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout = out
		os.Stderr = out
		startedFromTerminal = true
	}
}

// setupDesktop is called before anything else.
func setupDesktop() {
	attachConsole()
}

// whenReady is called once the port is listening.
func whenReady(url string) {
	// The address goes to the log first, so that it is in the file even if
	// the browser does not open.
	slog.Info("open this address in a browser", "url", url)

	// A person who double-clicked something gets a browser; a person who
	// typed a command gets the address printed.
	if !startedFromTerminal {
		openURL(url)
	}
}

// whenFatal is called when the program cannot start.
func whenFatal(message string) {
	slog.Error("cannot start", "err", message)
	tellUser("欢喜下载 / happy-downloader", message)
}
