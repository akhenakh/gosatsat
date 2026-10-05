//go:build linux

package main

import (
	"fmt"
	"os/exec"
)

// sendNotification posts a desktop notification via libnotify's notify-send.
// It works with any freedesktop notification daemon (GNOME, KDE, XFCE, …).
func sendNotification(title, body string) error {
	path, err := exec.LookPath("notify-send")
	if err != nil {
		return fmt.Errorf("notify-send not available: %w", err)
	}
	return exec.Command(path, "--app-name=SatSat", "--icon=dialog-information", title, body).Run()
}
