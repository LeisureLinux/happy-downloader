//go:build !windows

package main

import "errors"

// openAppWindow is not implemented outside Windows.
//
// WebView2 is a Windows component. Linux has WebKitGTK and macOS has WKWebView,
// and each of them would be a different implementation with its own
// dependencies; the browser the program already opens is the answer there, and
// on a server there is nobody at the screen to open a window for.
func openAppWindow(url string) error {
	return errors.New("this build has no window of its own")
}
