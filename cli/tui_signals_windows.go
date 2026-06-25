//go:build windows

package main

import "os"

func tuiExitSignals() []os.Signal {
	return nil
}
