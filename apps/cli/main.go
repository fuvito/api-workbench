package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags="-X main.version=v0.1.0".
var version = "dev"

// exitError is returned from cobra RunE to signal a non-zero exit code.
// Using a sentinel avoids calling os.Exit inside a handler, which would
// bypass any future deferred cleanup.
type exitError struct{ code int }

func (e *exitError) Error() string { return "" }

// newRootCmd is the whole command tree, as main runs it. The test of the detailed CLI guide walks
// it, so no command can be added without a page.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "clrsig",
		Short:         "Clear Signal — API Contract Validation and Spec Conformance Testing",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	addTLSFlags(root)
	addConfigFlags(root)
	root.AddCommand(runCmd())
	root.AddCommand(requestCmd())
	root.AddCommand(validateCmd())
	root.AddCommand(generateCmd())
	root.AddCommand(conformCmd())
	root.AddCommand(diffCmd())
	root.AddCommand(coverageCmd())
	root.AddCommand(impactCmd())
	root.AddCommand(mcpCmd())
	root.AddCommand(telemetryCmd())
	root.AddCommand(bundleCmd())
	root.AddCommand(importCmd())
	root.AddCommand(initCmd())
	root.AddCommand(configCmd())
	return root
}

func main() {
	root := newRootCmd()

	if err := root.Execute(); err != nil {
		var xe *exitError
		if errors.As(err, &xe) {
			os.Exit(xe.code)
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(2)
	}
}
