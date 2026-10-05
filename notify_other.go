//go:build !linux && !darwin && !windows

package main

// sendNotification is a no-op on platforms without a supported notifier.
func sendNotification(title, body string) error { return nil }
