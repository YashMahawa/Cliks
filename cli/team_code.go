package main

import (
	"fmt"
	"regexp"
	"strings"
)

var teamCodePattern = regexp.MustCompile(`^CLIK-[A-Z0-9]{6}$`)

func normalizeTeamCode(value string) (string, error) {
	code := strings.ToUpper(strings.TrimSpace(value))
	if code == "CLIK-LOCAL" || teamCodePattern.MatchString(code) {
		return code, nil
	}
	return "", fmt.Errorf("invalid team code %q; expected CLIK-XXXXXX", value)
}
