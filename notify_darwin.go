//go:build darwin

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// sendNotification posts a notification through the macOS Notification Center
// using osascript.
func sendNotification(title, body string) error {
	script := fmt.Sprintf("display notification %s with title %s",
		appleScriptString(body), appleScriptString(title))
	return exec.Command("osascript", "-e", script).Run()
}

// appleScriptString renders s as an AppleScript double-quoted string literal.
func appleScriptString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
