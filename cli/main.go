package main

import (
	"context"
	"os"

	"github.com/SUSE/metabroker/cli/cmd"
)

func main() {
	ctx := context.Background()
	if err := cmd.NewDefaultRootCmd().ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
