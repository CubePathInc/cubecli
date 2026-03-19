package output

import (
	"fmt"

	"github.com/fatih/color"
)

var (
	green  = color.New(color.FgGreen)
	red    = color.New(color.FgRed)
	yellow = color.New(color.FgYellow)
	cyan   = color.New(color.FgCyan)
	dim    = color.New(color.Faint)
)

func PrintSuccess(msg string) {
	green.Printf("✓ %s\n", msg)
}

func PrintError(msg string) {
	red.Printf("✗ %s\n", msg)
}

func PrintWarning(msg string) {
	yellow.Printf("⚠ %s\n", msg)
}

func PrintInfo(msg string) {
	cyan.Printf("ℹ %s\n", msg)
}

func FormatStatus(status string) string {
	switch status {
	case "active", "running", "healthy", "enabled", "verified":
		return green.Sprint(status)
	case "stopped", "paused", "pending", "provisioning", "inactive":
		return dim.Sprint(status)
	case "error", "failed", "unhealthy":
		return red.Sprint(status)
	default:
		return fmt.Sprint(status)
	}
}
