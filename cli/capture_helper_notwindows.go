//go:build !windows

package main

import "fmt"

func runWindowsCaptureHelper(args []string) error {
	fmt.Println("ready")
	return nil
}
