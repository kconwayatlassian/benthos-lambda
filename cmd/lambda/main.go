package main

import (
	"context"
	"fmt"
	"os"

	serverfull "github.com/asecurityteam/serverfull/pkg"
	"github.com/asecurityteam/serverfull/pkg/domain"
	"github.com/asecurityteam/settings"
	"github.com/benthosdev/benthos-lambda/lib"
)

func main() {
	ctx := context.Background()
	// Logging and stats aggregation.
	handler, closeFn, err := lib.NewRuntime([]byte(os.Getenv("BENTHOS_CONFIG")))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	handlers := map[string]domain.Handler{
		"benthos": handler,
	}
	source, err := settings.NewEnvSource(os.Environ())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	rt, err := serverfull.NewStatic(ctx, source, handlers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if err := rt.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if err := closeFn(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
