//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// sendNotification posts a notification through the Windows notification area
// using a PowerShell NotifyIcon balloon (available in Windows PowerShell 5.1).
func sendNotification(title, body string) error {
	script := fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$n = New-Object System.Windows.Forms.NotifyIcon
$n.Icon = [System.Drawing.SystemIcons]::Information
$n.Visible = $true
$n.ShowBalloonTip(10000, '%s', '%s', [System.Windows.Forms.ToolTipIcon]::Info)
Start-Sleep -Seconds 8
$n.Dispose()`, psString(title), psString(body))
	return exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command", script).Run()
}

// psString escapes a value for a single-quoted PowerShell string literal.
func psString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
