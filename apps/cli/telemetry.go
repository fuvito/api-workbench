package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fuvito/api-workbench/packages/core/domain"
	"github.com/fuvito/api-workbench/packages/core/telemetry"
	"github.com/spf13/cobra"
)

// --- Telemetry helpers ---

func clrsigDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".clrsig")
}

func isTelemetryEnabled() bool {
	dir := clrsigDataDir()
	if dir == "" {
		return false
	}
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		return false
	}
	var s struct {
		TelemetryEnabled bool `json:"telemetryEnabled"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return false
	}
	return s.TelemetryEnabled
}

func newTelemetryClient() *telemetry.Client {
	return telemetry.New(isTelemetryEnabled(), clrsigDataDir(), version)
}

func writeTelemetrySetting(enabled bool) error {
	dir := clrsigDataDir()
	if dir == "" {
		return fmt.Errorf("could not determine home directory")
	}
	path := filepath.Join(dir, "settings.json")

	s := domain.DefaultSettings()
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	s.TelemetryEnabled = enabled

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func telemetryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telemetry",
		Short: "Manage opt-in usage telemetry",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "enable",
		Short: "Opt in to usage telemetry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := writeTelemetrySetting(true); err != nil {
				return fmt.Errorf("updating settings: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Telemetry enabled.")
			fmt.Fprint(cmd.OutOrStdout(), telemetry.Disclosure)
			fmt.Fprintln(cmd.OutOrStdout(), "Run 'clrsig telemetry disable' to opt out at any time.")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "disable",
		Short: "Opt out of usage telemetry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := writeTelemetrySetting(false); err != nil {
				return fmt.Errorf("updating settings: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Telemetry disabled.")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show current telemetry opt-in status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if isTelemetryEnabled() {
				fmt.Fprintln(cmd.OutOrStdout(), "Telemetry: enabled")
				fmt.Fprint(cmd.OutOrStdout(), telemetry.Disclosure)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Telemetry: disabled (run 'clrsig telemetry enable' to opt in)")
			}
			return nil
		},
	})

	return cmd
}
