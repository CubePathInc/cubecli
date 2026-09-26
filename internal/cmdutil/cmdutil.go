package cmdutil

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/CubePathInc/cubecli/internal/api"
	"github.com/spf13/cobra"
)

type contextKey string

const (
	ClientKey        contextKey = "api-client"
	ActiveProfileKey contextKey = "active-profile"
)

func GetClient(cmd *cobra.Command) *api.Client {
	return cmd.Context().Value(ClientKey).(*api.Client)
}

func GetActiveProfileName(cmd *cobra.Command) string {
	v, _ := cmd.Context().Value(ActiveProfileKey).(string)
	return v
}

func IsJSON(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("json")
	return v
}

func ConfirmAction(msg string) bool {
	fmt.Printf("%s [y/N]: ", msg)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return answer == "y" || answer == "yes"
	}
	return false
}

// ConfirmDefaultYes asks a yes/no question where Enter means yes.
func ConfirmDefaultYes(msg string) bool {
	fmt.Printf("%s [Y/n]: ", msg)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return answer == "" || answer == "y" || answer == "yes"
	}
	return false
}

// StdinIsTerminal reports whether stdin is interactive. Commands that used to
// read a token from stdin keep doing so when it is piped, so scripts do not
// suddenly open a browser.
func StdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func CheckForce(cmd *cobra.Command, msg string) bool {
	force, _ := cmd.Flags().GetBool("force")
	if force {
		return true
	}
	return ConfirmAction(msg)
}
