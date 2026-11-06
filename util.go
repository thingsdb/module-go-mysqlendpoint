package main

import (
	"fmt"
	"regexp"
	"strings"
)

var validNameRegex = regexp.MustCompile(`^[A-Za-z_][0-9A-Za-z_]{0,254}$`)

func isName(s string) bool {
	return validNameRegex.MatchString(s)
}

func cnScope(scope string) (string, error) {
	var name string

	if strings.Contains(scope, ":") {
		parts := strings.Split(scope, ":")
		name = parts[len(parts)-1]
	} else if strings.Contains(scope, "/") {
		parts := strings.Split(scope, "/")
		name = parts[len(parts)-1]
	} else {
		name = ""
	}

	if isName(name) {
		return name, nil
	}

	return "", fmt.Errorf("invalid (collection) scope name: %s", scope)
}
