package main

import (
	"errors"
	"os"

	"github.com/CubePathInc/cubecli/cmd"
	"github.com/CubePathInc/cubecli/internal/api"
	"github.com/CubePathInc/cubecli/internal/output"
)

func main() {
	if err := cmd.Execute(); err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) {
			output.PrintError(apiErr.Detail)
		} else {
			output.PrintError(err.Error())
		}
		os.Exit(1)
	}
}
