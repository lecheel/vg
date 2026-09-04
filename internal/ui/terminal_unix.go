//go:build !windows

package ui

import (
	"os"
	"os/signal"
	"syscall"
)

func watchWindowResize() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			updateTerminalSize()
		}
	}()
}
