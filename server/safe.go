package main

import (
	"log"
	"runtime/debug"
)

func recoverAndLog(scope string) {
	if recovered := recover(); recovered != nil {
		log.Printf("recovered panic in %s: %v\n%s", scope, recovered, debug.Stack())
	}
}

func runSafely(scope string, task func()) (recovered bool) {
	defer func() {
		if value := recover(); value != nil {
			recovered = true
			log.Printf("recovered panic in %s: %v\n%s", scope, value, debug.Stack())
		}
	}()
	task()
	return false
}
