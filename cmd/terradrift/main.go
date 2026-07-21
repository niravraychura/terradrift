package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/niravraychura/terradrift/internal/logger"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCommand(os.Stdout, os.Stderr).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func newRootCommand(stdout, stderr io.Writer) *cobra.Command {
	var logLevel string

	cmd := &cobra.Command{
		Use:           "terradrift",
		Short:         "Self-hosted Terraform drift detection",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			level, err := logger.ParseLevel(logLevel)
			if err != nil {
				return err
			}
			_ = logger.New(stderr, level)
			return nil
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level: debug, info, warn, error")
	cmd.AddCommand(newScanCommand(stdout))
	return cmd
}

func newScanCommand(stdout io.Writer) *cobra.Command {
	var directory string
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Validate a Terraform directory for drift scanning",
		RunE: func(cmd *cobra.Command, args []string) error {
			absDir, err := validateDirectory(directory)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintln(stdout, "TerraDrift scan initialized"); err != nil {
				return fmt.Errorf("write scan output: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "Terraform directory: %s\n", absDir); err != nil {
				return fmt.Errorf("write scan output: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&directory, "directory", "d", "", "Terraform directory to scan")
	return cmd
}

func validateDirectory(directory string) (string, error) {
	if directory == "" {
		return "", fmt.Errorf("terraform directory is required; provide --directory or -d")
	}
	absDir, err := filepath.Abs(directory)
	if err != nil {
		return "", fmt.Errorf("resolve terraform directory: %w", err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("terraform directory does not exist: %s", absDir)
		}
		return "", fmt.Errorf("inspect terraform directory %s: %w", absDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("terraform path is not a directory: %s", absDir)
	}
	return absDir, nil
}
