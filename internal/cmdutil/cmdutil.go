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

func CheckForce(cmd *cobra.Command, msg string) bool {
	force, _ := cmd.Flags().GetBool("force")
	if force {
		return true
	}
	return ConfirmAction(msg)
}
