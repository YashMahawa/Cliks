//go:build !linux

package main

import "os/exec"

func prepareAmbientCommand(cmd *exec.Cmd) {}

func cleanupOrphanAmbientPlayers() int { return 0 }
