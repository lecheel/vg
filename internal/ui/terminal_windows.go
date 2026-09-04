//go:build windows

package ui

func watchWindowResize() {
	// Windows does not support SIGWINCH.
}
